declare global {
  namespace App {}

  interface Window {
    ot?: {
      toast: (message: string, title?: string, options?: { variant?: string; placement?: string; duration?: number }) => void
    }
  }
}

export {}
