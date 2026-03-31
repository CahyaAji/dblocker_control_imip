import { mount } from 'svelte'
import './app.css'
import ActionLogPage from './lib/components/ActionLogPage.svelte'

const app = mount(ActionLogPage, {
  target: document.getElementById('app')!,
})

export default app
