import { mount } from 'svelte'
import App from './App.svelte'
import '@knadh/oat/oat.min.css'
import '@knadh/oat/oat.min.js'

mount(App, {
  target: document.getElementById('app')
})
