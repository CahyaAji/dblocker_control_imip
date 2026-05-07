import { mount } from 'svelte'
import './app.css'
import CameraPage from './lib/components/CameraPage.svelte'

const app = mount(CameraPage, {
    target: document.getElementById('app')!,
})

export default app
