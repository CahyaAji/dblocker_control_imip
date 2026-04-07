import { mount } from 'svelte'
import './app.css'
import DetectionsPage from './lib/components/DetectionsPage.svelte'

const app = mount(DetectionsPage, {
    target: document.getElementById('app')!,
})

export default app
