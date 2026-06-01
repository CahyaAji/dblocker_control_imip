import { mount } from 'svelte'
import './app.css'
import WhitelistPage from './lib/components/WhitelistPage.svelte'

const app = mount(WhitelistPage, {
    target: document.getElementById('app')!,
})

export default app
