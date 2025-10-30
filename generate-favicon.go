package main

import (
	"encoding/base64"
	"fmt"
	"os"
)

// GenerateFavicon creates a gold-themed favicon
func GenerateFavicon() {
	// SVG favicon with gold gradient
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <defs>
    <linearGradient id="gold" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" style="stop-color:#f4cf47;stop-opacity:1" />
      <stop offset="100%" style="stop-color:#d4af37;stop-opacity:1" />
    </linearGradient>
  </defs>
  <rect width="32" height="32" rx="6" fill="#0a0a0a"/>
  <text x="16" y="22" font-family="Arial Black" font-size="18" font-weight="bold" 
        text-anchor="middle" fill="url(#gold)">W</text>
</svg>`

	// Save SVG favicon
	err := os.WriteFile("web/favicon.svg", []byte(svgContent), 0644)
	if err != nil {
		fmt.Printf("Error creating favicon.svg: %v\n", err)
		return
	}

	// Create base64 data URL for embedding
	dataURL := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svgContent))
	
	// Also create a PNG version (16x16) - base64 encoded minimal PNG
	pngBase64 := "iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAABHNCSVQICAgIfAhkiAAAAAlwSFlzAAAA7AAAAOwBeShxvQAAABl0RVh0U29mdHdhcmUAd3d3Lmlua3NjYXBlLm9yZ5vuPBoAAADWSURBVDiNpdMxSgNBFMbx3xtBEETQwsLCQhAsBBHBysJKC0+QA3gCL5ATeBIvYGFhYWFhIYiIoGChiIWF4FqMb2GyyS6bXfzg8Zhv5v/NvHkzVFXtYRenjT6OUPcPsIk5bEUuYx4bVYEOsIRFzMbeRBaxtF3EKh6xihu84TKP9wvYR4593ONrAucRf8BdHu8EvEN+K7yOOgmMVkwgT4BRDGJABXs4CcII1nGFKZygiy6mcY11jJQNUIWr+MD7GAa0wmEM6Ic5NEaQxrCGBXyih368A9Q/8A2j5R7XdE1XUAAAAABJRU5ErkJggg=="
	
	pngData, _ := base64.StdEncoding.DecodeString(pngBase64)
	err = os.WriteFile("web/favicon.png", pngData, 0644)
	if err != nil {
		fmt.Printf("Error creating favicon.png: %v\n", err)
		return
	}

	// Also save the data URL for use in HTML
	err = os.WriteFile("web/favicon-data-url.txt", []byte(dataURL), 0644)
	if err != nil {
		fmt.Printf("Error saving data URL: %v\n", err)
		return
	}

	fmt.Println("✅ Favicons generated successfully:")
	fmt.Println("  - web/favicon.svg")
	fmt.Println("  - web/favicon.png")
	fmt.Println("  - web/favicon-data-url.txt")
}

// Call this in main server initialization
func init() {
	// Ensure web directory exists
	os.MkdirAll("web", 0755)
	GenerateFavicon()
}
