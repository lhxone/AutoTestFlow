declare module 'three-spritetext' {
  import type { Object3D } from 'three'

  export default class SpriteText extends Object3D {
    constructor(text?: string)
    text: string
    color: string
    textHeight: number
    backgroundColor: string
    padding: number
  }
}
