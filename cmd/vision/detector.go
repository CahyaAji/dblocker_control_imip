package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"math"
	"os"
	"sort"
	"sync"

	"github.com/fogleman/gg"
	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/image/draw"
)

// ── ONNX Runtime singleton ──────────────────────────────────────────────────

var (
	ortInitOnce sync.Once
	ortInitErr  error
)

// initORT initialises the ONNX Runtime once per process.
// ORT_LIBRARY_PATH may override the shared library location.
func initORT() error {
	ortInitOnce.Do(func() {
		if libPath := os.Getenv("ORT_LIBRARY_PATH"); libPath != "" {
			ort.SetSharedLibraryPath(libPath)
		}
		ortInitErr = ort.InitializeEnvironment()
	})
	return ortInitErr
}

// ── Detector ────────────────────────────────────────────────────────────────

// YOLOv8 ONNX export expects input shape [1,3,640,640] float32 (RGB, normalised 0-1).
// Output shape is [1, 4+nc, 8400] where the first 4 rows are cx,cy,w,h (model space)
// and the next nc rows are per-class scores.
const (
	yoloInputSize = 640
)

// Detection is a single bounding-box result in original image coordinates.
type Detection struct {
	X1, Y1, X2, Y2 float32
	Confidence     float32
	ClassID        int
	ClassName      string
}

// Detector runs YOLOv8 inference using ONNX Runtime.
// Run is safe for concurrent use by multiple goroutines: an internal mutex
// serialises access to the underlying ORT session and tensors (which are
// single-instance and not thread-safe).
type Detector struct {
	mu         sync.Mutex
	session    *ort.AdvancedSession
	inTensor   *ort.Tensor[float32]
	outTensor  *ort.Tensor[float32]
	classNames []string
	confTh     float32
	iouTh      float32
	numClasses int
	outBoxes   int // anchors per class row, e.g. 8400
}

// NewDetector loads a YOLOv8 ONNX model.
func NewDetector(modelPath string, classNames []string, confTh, iouTh float32) (*Detector, error) {
	if err := initORT(); err != nil {
		return nil, fmt.Errorf("init onnxruntime: %w", err)
	}

	// Probe a small set of common YOLOv8 anchor counts.
	// 8400 = 640x640 default; the model is fixed so we try this first.
	const numAnchors = 8400
	nc := len(classNames)

	inShape := ort.NewShape(1, 3, yoloInputSize, yoloInputSize)
	inTensor, err := ort.NewEmptyTensor[float32](inShape)
	if err != nil {
		return nil, fmt.Errorf("create input tensor: %w", err)
	}
	outShape := ort.NewShape(1, int64(4+nc), int64(numAnchors))
	outTensor, err := ort.NewEmptyTensor[float32](outShape)
	if err != nil {
		inTensor.Destroy()
		return nil, fmt.Errorf("create output tensor: %w", err)
	}

	opts, err := buildSessionOptions()
	if err != nil {
		inTensor.Destroy()
		outTensor.Destroy()
		return nil, err
	}
	defer opts.Destroy()

	session, err := ort.NewAdvancedSession(modelPath,
		[]string{"images"}, []string{"output0"},
		[]ort.Value{inTensor}, []ort.Value{outTensor},
		opts)
	if err != nil {
		inTensor.Destroy()
		outTensor.Destroy()
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &Detector{
		session:    session,
		inTensor:   inTensor,
		outTensor:  outTensor,
		classNames: classNames,
		confTh:     confTh,
		iouTh:      iouTh,
		numClasses: nc,
		outBoxes:   numAnchors,
	}, nil
}

// buildSessionOptions creates ORT session options and optionally enables the
// CUDA execution provider when DETECT_DEVICE=cuda is set in the environment.
// The caller must call opts.Destroy() when the session has been created.
func buildSessionOptions() (*ort.SessionOptions, error) {
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("create session options: %w", err)
	}
	if os.Getenv("DETECT_DEVICE") == "cuda" {
		cudaOpts, err := ort.NewCUDAProviderOptions()
		if err != nil {
			opts.Destroy()
			return nil, fmt.Errorf("create CUDA provider options: %w", err)
		}
		defer cudaOpts.Destroy()
		if err := opts.AppendExecutionProviderCUDA(cudaOpts); err != nil {
			opts.Destroy()
			return nil, fmt.Errorf("append CUDA execution provider: %w", err)
		}
		log.Println("Detector: CUDA execution provider enabled (GPU)")
	} else {
		log.Println("Detector: using CPU execution provider")
	}
	return opts, nil
}

// Close releases ORT resources.
func (d *Detector) Close() {
	if d.session != nil {
		d.session.Destroy()
	}
	if d.inTensor != nil {
		d.inTensor.Destroy()
	}
	if d.outTensor != nil {
		d.outTensor.Destroy()
	}
}

// Run runs inference on a JPEG frame and returns detections in original image coordinates.
func (d *Detector) Run(jpegFrame []byte) ([]Detection, image.Image, error) {
	// The ORT session and the input/output tensors are reused across calls,
	// so callers from different cameras must not run concurrently.
	d.mu.Lock()
	defer d.mu.Unlock()

	img, err := jpeg.Decode(bytes.NewReader(jpegFrame))
	if err != nil {
		return nil, nil, fmt.Errorf("decode jpeg: %w", err)
	}
	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()

	// Letterbox to 640x640 with grey padding.
	scale := math.Min(float64(yoloInputSize)/float64(srcW), float64(yoloInputSize)/float64(srcH))
	newW := int(math.Round(float64(srcW) * scale))
	newH := int(math.Round(float64(srcH) * scale))
	padX := (yoloInputSize - newW) / 2
	padY := (yoloInputSize - newH) / 2

	resized := image.NewRGBA(image.Rect(0, 0, yoloInputSize, yoloInputSize))
	// Fill with neutral grey (114/255 like ultralytics).
	grey := color.RGBA{114, 114, 114, 255}
	draw.Draw(resized, resized.Bounds(), image.NewUniform(grey), image.Point{}, draw.Src)
	target := image.Rect(padX, padY, padX+newW, padY+newH)
	draw.CatmullRom.Scale(resized, target, img, bounds, draw.Over, nil)

	// Pack into CHW float32 [1,3,640,640].
	data := d.inTensor.GetData()
	plane := yoloInputSize * yoloInputSize
	for y := 0; y < yoloInputSize; y++ {
		for x := 0; x < yoloInputSize; x++ {
			i := y*resized.Stride + x*4
			idx := y*yoloInputSize + x
			data[idx] = float32(resized.Pix[i]) / 255.0           // R
			data[plane+idx] = float32(resized.Pix[i+1]) / 255.0   // G
			data[2*plane+idx] = float32(resized.Pix[i+2]) / 255.0 // B
		}
	}

	if err := d.session.Run(); err != nil {
		return nil, img, fmt.Errorf("session run: %w", err)
	}

	out := d.outTensor.GetData()
	dets := d.parseOutput(out, scale, padX, padY, srcW, srcH)
	return dets, img, nil
}

// parseOutput decodes the YOLOv8 raw output tensor, applies confidence
// thresholding and per-class NMS, and converts boxes from letterbox-640 space
// back to the original image.
func (d *Detector) parseOutput(out []float32, scale float64, padX, padY, srcW, srcH int) []Detection {
	// Layout: [4+nc, 8400] flattened. Row r, col c is at out[r*8400 + c].
	rows := 4 + d.numClasses
	cols := d.outBoxes
	if len(out) < rows*cols {
		return nil
	}

	candidates := make([]Detection, 0, 64)
	for c := 0; c < cols; c++ {
		// Find best class for this anchor.
		bestScore := float32(0)
		bestClass := -1
		for k := 0; k < d.numClasses; k++ {
			s := out[(4+k)*cols+c]
			if s > bestScore {
				bestScore = s
				bestClass = k
			}
		}
		if bestScore < d.confTh || bestClass < 0 {
			continue
		}
		cx := out[0*cols+c]
		cy := out[1*cols+c]
		w := out[2*cols+c]
		h := out[3*cols+c]
		// Box in letterbox-640 space.
		x1 := cx - w/2
		y1 := cy - h/2
		x2 := cx + w/2
		y2 := cy + h/2
		// Reverse letterbox: subtract pad, divide by scale.
		ox1 := (float64(x1) - float64(padX)) / scale
		oy1 := (float64(y1) - float64(padY)) / scale
		ox2 := (float64(x2) - float64(padX)) / scale
		oy2 := (float64(y2) - float64(padY)) / scale
		// Clamp to image bounds.
		if ox1 < 0 {
			ox1 = 0
		}
		if oy1 < 0 {
			oy1 = 0
		}
		if ox2 > float64(srcW) {
			ox2 = float64(srcW)
		}
		if oy2 > float64(srcH) {
			oy2 = float64(srcH)
		}
		name := ""
		if bestClass < len(d.classNames) {
			name = d.classNames[bestClass]
		}
		candidates = append(candidates, Detection{
			X1: float32(ox1), Y1: float32(oy1),
			X2: float32(ox2), Y2: float32(oy2),
			Confidence: bestScore,
			ClassID:    bestClass,
			ClassName:  name,
		})
	}

	return nms(candidates, d.iouTh)
}

// nms performs per-class non-maximum suppression.
func nms(dets []Detection, iouTh float32) []Detection {
	if len(dets) == 0 {
		return dets
	}
	// Sort by confidence desc.
	sort.Slice(dets, func(i, j int) bool { return dets[i].Confidence > dets[j].Confidence })

	suppressed := make([]bool, len(dets))
	keep := make([]Detection, 0, len(dets))
	for i := range dets {
		if suppressed[i] {
			continue
		}
		keep = append(keep, dets[i])
		for j := i + 1; j < len(dets); j++ {
			if suppressed[j] || dets[j].ClassID != dets[i].ClassID {
				continue
			}
			if iou(dets[i], dets[j]) > iouTh {
				suppressed[j] = true
			}
		}
	}
	return keep
}

func iou(a, b Detection) float32 {
	xx1 := maxF(a.X1, b.X1)
	yy1 := maxF(a.Y1, b.Y1)
	xx2 := minF(a.X2, b.X2)
	yy2 := minF(a.Y2, b.Y2)
	w := maxF(0, xx2-xx1)
	h := maxF(0, yy2-yy1)
	inter := w * h
	areaA := (a.X2 - a.X1) * (a.Y2 - a.Y1)
	areaB := (b.X2 - b.X1) * (b.Y2 - b.Y1)
	union := areaA + areaB - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// ── Annotation ──────────────────────────────────────────────────────────────

// AnnotateAndEncode draws bounding boxes onto img and returns a JPEG.
func AnnotateAndEncode(img image.Image, dets []Detection, jpegQuality int) ([]byte, error) {
	dc := gg.NewContextForImage(img)
	dc.SetLineWidth(2)
	for _, det := range dets {
		dc.SetColor(color.RGBA{0, 0, 255, 255}) // red box (BGR-style red in OpenCV)
		dc.DrawRectangle(float64(det.X1), float64(det.Y1), float64(det.X2-det.X1), float64(det.Y2-det.Y1))
		dc.Stroke()

		label := fmt.Sprintf("%s %.0f%%", det.ClassName, det.Confidence*100)
		dc.SetColor(color.RGBA{255, 255, 255, 255})
		dc.DrawString(label, float64(det.X1), float64(det.Y1)-4)
	}
	out := dc.Image()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── Class names ─────────────────────────────────────────────────────────────

// CocoClasses are the 80 COCO class names used by the default YOLOv8n weights.
var CocoClasses = []string{
	"person", "bicycle", "car", "motorcycle", "airplane", "bus", "train", "truck", "boat",
	"traffic light", "fire hydrant", "stop sign", "parking meter", "bench", "bird", "cat",
	"dog", "horse", "sheep", "cow", "elephant", "bear", "zebra", "giraffe", "backpack",
	"umbrella", "handbag", "tie", "suitcase", "frisbee", "skis", "snowboard", "sports ball",
	"kite", "baseball bat", "baseball glove", "skateboard", "surfboard", "tennis racket",
	"bottle", "wine glass", "cup", "fork", "knife", "spoon", "bowl", "banana", "apple",
	"sandwich", "orange", "broccoli", "carrot", "hot dog", "pizza", "donut", "cake",
	"chair", "couch", "potted plant", "bed", "dining table", "toilet", "tv", "laptop",
	"mouse", "remote", "keyboard", "cell phone", "microwave", "oven", "toaster", "sink",
	"refrigerator", "book", "clock", "vase", "scissors", "teddy bear", "hair drier", "toothbrush",
}

// avoid unused-import errors when log is not used elsewhere in this file
var _ = log.Printf
