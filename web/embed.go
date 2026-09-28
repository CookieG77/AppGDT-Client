// Embeds the HTML templates and static files in the binary, so the client
// can be deployed as a single executable

package web

import "embed"

//go:embed templates static
var FS embed.FS
