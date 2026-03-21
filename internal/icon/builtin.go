package icon

// builtinIcon returns a pre-defined SVG for TS3 built-in icon IDs (1-999).
// These icons are embedded in the TS3 desktop client and cannot be
// downloaded via File Transfer.
func builtinIcon(iconID uint32) *CachedItem {
	svg, ok := builtinSVGs[iconID]
	if !ok {
		svg = fallbackSVG
	}
	return &CachedItem{
		Body:        []byte(svg),
		ContentType: "image/svg+xml",
	}
}

// TS3 built-in icon ID → SVG mapping.
// Icons are 16×16 simple shapes matching the TS3 desktop client style.
var builtinSVGs = map[uint32]string{
	// 100: Channel Admin (blue shield with wrench)
	100: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16">` +
		`<path d="M8 1L2 4v4c0 3.3 2.6 6.1 6 7 3.4-.9 6-3.7 6-7V4L8 1z" fill="#4a86c8"/>` +
		`<path d="M7 5.5l-2 2 1 1 1-1 3 3 1-1-4-4z" fill="#fff"/>` +
		`</svg>`,

	// 200: Member (green person)
	200: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16">` +
		`<circle cx="8" cy="5" r="2.5" fill="#5b9a3a"/>` +
		`<path d="M4 13c0-2.2 1.8-4 4-4s4 1.8 4 4H4z" fill="#5b9a3a"/>` +
		`</svg>`,

	// 300: Server Admin (gold shield with star)
	300: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16">` +
		`<path d="M8 1L2 4v4c0 3.3 2.6 6.1 6 7 3.4-.9 6-3.7 6-7V4L8 1z" fill="#d4a017"/>` +
		`<path d="M8 4l1 2h2l-1.5 1.5.5 2L8 8.5 5.9 9.5l.6-2L5 6h2l1-2z" fill="#fff"/>` +
		`</svg>`,

	// 400: Talk Power (microphone)
	400: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16">` +
		`<rect x="6" y="2" width="4" height="7" rx="2" fill="#888"/>` +
		`<path d="M4 8a4 4 0 0 0 8 0" fill="none" stroke="#888" stroke-width="1.5"/>` +
		`<line x1="8" y1="12" x2="8" y2="14" stroke="#888" stroke-width="1.5"/>` +
		`<line x1="6" y1="14" x2="10" y2="14" stroke="#888" stroke-width="1.5"/>` +
		`</svg>`,

	// 500: Moderator (blue person with star)
	500: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16">` +
		`<circle cx="8" cy="5" r="2.5" fill="#4a86c8"/>` +
		`<path d="M4 13c0-2.2 1.8-4 4-4s4 1.8 4 4H4z" fill="#4a86c8"/>` +
		`<circle cx="12" cy="4" r="2" fill="#d4a017"/>` +
		`</svg>`,

	// 600: Music Bot (purple music note)
	600: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16">` +
		`<path d="M11 2v8.3A2.5 2.5 0 1 1 9 8V4.5L6 5.5v5.8a2.5 2.5 0 1 1-2-2.5V3l7-1z" fill="#8b5cf6"/>` +
		`</svg>`,
}

// fallbackSVG is used for unrecognized built-in icon IDs.
var fallbackSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16">` +
	`<circle cx="8" cy="8" r="6" fill="#888"/>` +
	`<text x="8" y="11" text-anchor="middle" fill="#fff" font-size="8" font-family="sans-serif">?</text>` +
	`</svg>`
