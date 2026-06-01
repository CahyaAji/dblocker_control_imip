### Port yg digunakan

#### drone blocker
1. mqtt, 1883 (komunikasi utama ke server)
2. udp, 51515 (dapat digunakan jika broker mqtt down)

#### detector
1. tcp, 5555

#### Camera
1. rtsp, 554 (stream video)
2. http, 80 (PTZ control)


#### PC
1. http, 8080 (main API)
2. tcp, 5432 (database)
3. http, 8090 (vision api) 