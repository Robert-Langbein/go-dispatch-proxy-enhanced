// web_templates.go
package main

import (
	"html/template"
	"io/ioutil"
	"path/filepath"
	"strings"
)

/*
Load and return login HTML template
*/
func getLoginHTML(errorMsg string) string {
	templatePath := filepath.Join("web", "templates", "login.html")
	content, err := ioutil.ReadFile(templatePath)
	if err != nil {
		// Fallback to minimal inline template
		return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go Dispatch Proxy Enhanced - Login</title>
    <link rel="stylesheet" href="/static/css/main.css">
    <link rel="stylesheet" href="/static/css/login.css">
</head>
<body class="login-page">
    <div class="login-container">
        <div class="login-box">
            <h1>🚀 Go Dispatch Proxy Enhanced</h1>
            <p>Enhanced Load Balancing Web Interface</p>
            ` + func() string {
				if errorMsg != "" {
					return `<div class="error">` + errorMsg + `</div>`
				}
				return ""
			}() + `
            <form method="POST" action="/login">
                <div class="form-group">
                    <label for="username">Username:</label>
                    <input type="text" id="username" name="username" required autofocus>
                </div>
                <div class="form-group">
                    <label for="password">Password:</label>
                    <input type="password" id="password" name="password" required>
                </div>
                <button type="submit" class="btn btn-primary">Login</button>
            </form>
            <div class="info">
                <small>Default credentials: admin/admin<br>
                Set WEB_USERNAME and WEB_PASSWORD environment variables to customize</small>
            </div>
        </div>
    </div>
</body>
</html>`
	}
	
	// Parse and execute template with error message
	tmpl, err := template.New("login").Parse(string(content))
	if err != nil {
		return string(content) // Return raw content if parsing fails
	}
	
	var result strings.Builder
	data := struct {
		ErrorMsg string
	}{
		ErrorMsg: errorMsg,
	}
	
	err = tmpl.Execute(&result, data)
	if err != nil {
		return string(content) // Return raw content if execution fails
	}
	
	return result.String()
}

/*
Load and return dashboard HTML template content
*/
func getDashboardHTML() string {
	templatePath := filepath.Join("web", "templates", "dashboard.html")
	content, err := ioutil.ReadFile(templatePath)
	if err != nil {
		// Fallback template
		return `<!DOCTYPE html>
<html><head><title>Dashboard Error</title></head>
<body><h1>Template Error</h1><p>Could not load dashboard template: ` + err.Error() + `</p></body></html>`
	}
	return string(content)
}

/*
Load CSS file content
*/
func getCSS() string {
	// Load main CSS
	mainCSS := loadStaticFile(filepath.Join("web", "static", "css", "main.css"))
	loginCSS := loadStaticFile(filepath.Join("web", "static", "css", "login.css"))
	dashboardCSS := loadStaticFile(filepath.Join("web", "static", "css", "dashboard.css"))
	settingsCSS := loadStaticFile(filepath.Join("web", "static", "css", "settings.css"))
	
	// Combine all CSS files
	return mainCSS + "\n\n" + loginCSS + "\n\n" + dashboardCSS + "\n\n" + settingsCSS
}

/*
Load JavaScript file content
*/
func getJavaScript() string {
	// Load main JS
	mainJS := loadStaticFile(filepath.Join("web", "static", "js", "main.js"))
	dashboardJS := loadStaticFile(filepath.Join("web", "static", "js", "dashboard.js"))
	settingsJS := loadStaticFile(filepath.Join("web", "static", "js", "settings.js"))
	
	// Combine all JS files
	return mainJS + "\n\n" + dashboardJS + "\n\n" + settingsJS
}

/*
Load and return settings HTML template content
*/
func getSettingsHTML() string {
	templatePath := filepath.Join("web", "templates", "settings.html")
	content, err := ioutil.ReadFile(templatePath)
	if err != nil {
		// Fallback template
		return `<!DOCTYPE html>
<html><head><title>Settings Error</title></head>
<body><h1>Template Error</h1><p>Could not load settings template: ` + err.Error() + `</p></body></html>`
	}
	return string(content)
}

/*
Helper function to load static files
*/
func loadStaticFile(path string) string {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "/* Error loading " + path + ": " + err.Error() + " */"
	}
	return string(content)
} 