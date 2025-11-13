# Church Application RWeb Framework Migration Plan

## Overview
This document provides a comprehensive plan for migrating the Church CMS application from Echo to the RWeb framework. The application is a sophisticated content management system with modules, authentication, session management, and media serving capabilities.

## Key Differences: Echo vs RWeb

### 1. Server Initialization
**Echo:**
```go
e := echo.New()
e.HideBanner = true
e.Start(":8080")
```

**RWeb:**
```go
s := rweb.NewServer(rweb.ServerOptions{
    Address: ":8080",  // Use ":8080" format (not "localhost:8080") for Docker compatibility
    Verbose: true,
    Debug: false,
    TLS: rweb.TLSCfg{
        UseTLS:   false,
        KeyFile:  "certs/localhost.key",
        CertFile: "certs/localhost.crt",
    },
})
s.Run()
```

### 2. Handler Signature
**Echo:**
```go
func handler(c echo.Context) error
```

**RWeb:**
```go
func handler(ctx rweb.Context) error
```

### 3. Parameter Access
**Echo:**
```go
// Path parameters
c.Param("id")
// Query parameters
c.QueryParam("filter")
// Form values
c.FormValue("name")
```

**RWeb:**
```go
// Path parameters
ctx.Request().PathParam("id")     // Preferred method
ctx.Request().Param("id")         // Alias for PathParam
// Query parameters
ctx.Request().QueryParam("filter")
// Form values
ctx.Request().FormValue("name")
```

### 4. Request Body Handling
**Echo:**
```go
var payload MyStruct
c.Bind(&payload)
```

**RWeb:**
```go
body, err := io.ReadAll(ctx.Request().Body())
if err != nil {
    return err
}
var payload MyStruct
err = json.Unmarshal(body, &payload)
```

### 5. Response Methods
**Echo:**
```go
c.JSON(200, data)
c.HTML(200, htmlContent)
c.String(200, "text")
c.File("path/to/file")
```

**RWeb:**
```go
ctx.WriteJSON(data)              // Auto-sets 200 status
ctx.Status(201).WriteJSON(data)  // Custom status
ctx.WriteHTML(htmlContent)
ctx.WriteString("text")
ctx.WriteText("text")           // Alias for WriteString
rweb.File(ctx, "filename", data)
```

### 6. Middleware
**Echo:**
```go
e.Use(middleware.Logger())
e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // pre-processing
        err := next(c)
        // post-processing
        return err
    }
})
```

**RWeb:**
```go
s.Use(rweb.RequestInfo)  // Built-in request logger
s.Use(func(ctx rweb.Context) error {
    // pre-processing
    defer func() {
        // post-processing (using defer)
    }()
    return ctx.Next()  // MUST call Next() to continue chain
})
```

### 7. Groups and Routing
**Echo:**
```go
api := e.Group("/api")
api.Use(authMiddleware)
v1 := api.Group("/v1")
v1.GET("/users", getUsers)
```

**RWeb:**
```go
api := s.Group("/api")
v1 := api.Group("/v1")
// Middleware can be applied to groups
users := v1.Group("/users", authMiddleware)
users.Get("/", listUsers)
users.Get("/:id", getUser)
// Or inline middleware for groups
admin := s.Group("/admin", authMiddleware, adminMiddleware)
```

### 8. Static Files
**Echo:**
```go
e.Static("/static", "assets")
```

**RWeb:**
```go
// StaticFiles(urlPrefix, localPath, stripPrefixSegments)
s.StaticFiles("/static/", "assets", 1)  // Strip "/static" from URL
s.StaticFiles("/css/", "assets/css", 1)  // More specific mappings
```

### 9. File Uploads
**Echo:**
```go
file, err := c.FormFile("file")
src, err := file.Open()
```

**RWeb:**
```go
file, header, err := ctx.Request().GetFormFile("file")
// file is already opened, just use it
defer file.Close()
data, err := io.ReadAll(file)
```

## Authentication and Session Management

### Context Data Storage Pattern
RWeb provides request-scoped data storage through the Context interface. This is the recommended pattern for authentication state:

```go
// Authentication Middleware
func authMiddleware(ctx rweb.Context) error {
    authHeader := ctx.Request().Header("Authorization")
    
    // Validate token (JWT, session, etc.)
    if isValidToken(authHeader) {
        // Store authentication state in context
        ctx.Set("isLoggedIn", true)
        ctx.Set("userId", extractUserId(authHeader))
        ctx.Set("username", extractUsername(authHeader))
        ctx.Set("isAdmin", checkAdminRole(authHeader))
    }
    
    return ctx.Next()
}

// Protected Route Handler
func protectedHandler(ctx rweb.Context) error {
    // Check authentication
    if !ctx.Has("isLoggedIn") || !ctx.Get("isLoggedIn").(bool) {
        return ctx.Status(401).WriteJSON(map[string]string{
            "error": "Authentication required",
        })
    }
    
    // Access user data
    userId := ctx.Get("userId").(string)
    username := ctx.Get("username").(string)
    
    // ... handle request
}

// Admin-only Route
func adminHandler(ctx rweb.Context) error {
    if !ctx.Has("isAdmin") || !ctx.Get("isAdmin").(bool) {
        return ctx.Status(403).WriteJSON(map[string]string{
            "error": "Admin access required",
        })
    }
    
    // ... handle admin request
}
```

### Session Management Example
```go
// Session middleware (runs after auth middleware)
func sessionMiddleware(ctx rweb.Context) error {
    if ctx.Has("isLoggedIn") && ctx.Get("isLoggedIn").(bool) {
        // Load session data from database/cache
        sessionData := loadSession(ctx.Get("userId").(string))
        ctx.Set("session", sessionData)
    }
    return ctx.Next()
}

// Logout handler
func logoutHandler(ctx rweb.Context) error {
    // Clear all auth-related context data
    ctx.Delete("isLoggedIn")
    ctx.Delete("userId")
    ctx.Delete("username")
    ctx.Delete("isAdmin")
    ctx.Delete("session")
    
    // Also clear session from storage
    clearSession(ctx.Get("userId").(string))
    
    return ctx.WriteString("Logged out successfully")
}
```

## Server-Sent Events (SSE)

### Basic SSE Setup
```go
// Create event channel
eventsChan := make(chan any, 10)

// Setup SSE endpoint (recommended method)
s.Get("/events", func(c rweb.Context) error {
    return s.SetupSSE(c, eventsChan)
})

// Or use convenience handler
s.Get("/events2", s.SSEHandler(eventsChan))

// Send events from anywhere in your application
eventsChan <- "event data"
eventsChan <- map[string]any{"type": "update", "data": "value"}
```

### SSE with Authentication
```go
s.Get("/user-events", authMiddleware, func(ctx rweb.Context) error {
    userId := ctx.Get("userId").(string)
    
    // Create user-specific event channel
    userEventsChan := getUserEventChannel(userId)
    
    return s.SetupSSE(ctx, userEventsChan)
})
```

## HTML Generation with Element

RWeb works seamlessly with the `github.com/rohanthewiz/element` package for HTML generation:

```go
import "github.com/rohanthewiz/element"

func htmlHandler(ctx rweb.Context) error {
    b := element.NewBuilder()
    
    b.Html().R(
        b.Head().R(
            b.Title().T("My Page"),
            b.Style().T("body { font-family: sans-serif; }"),
        ),
        b.Body().R(
            b.H1().T("Welcome"),
            b.DivClass("content").R(
                b.P().T("Hello, world!"),
                b.UlClass("list").R(
                    element.ForEach([]string{"item1", "item2"}, func(item string) {
                        b.Li().T(item)
                    }),
                ),
            ),
        ),
    )
    
    return ctx.WriteHTML(b.String())
}
```

## Error Handling with Serr

Use `github.com/rohanthewiz/serr` for consistent error handling:

```go
import "github.com/rohanthewiz/serr"

func handler(ctx rweb.Context) error {
    data, err := fetchData()
    if err != nil {
        return serr.Wrap(err, "failed to fetch data")
    }
    
    if data == nil {
        return serr.New("no data found")
    }
    
    return ctx.WriteJSON(data)
}
```

## Migration Checklist

1. **Dependencies**
   - Replace `github.com/labstack/echo/v4` with `github.com/rohanthewiz/rweb`
   - Where element is not already used, use  `github.com/rohanthewiz/element` for HTML generation
   - Add `github.com/rohanthewiz/serr` for error handling
   - Add `github.com/rohanthewiz/logger` for logging, but in most cases just wrap the error with serr and return to the parent

2. **Server Setup**
   - Replace Echo instance with RWeb server
   - Update server configuration options
   - Migrate TLS settings if applicable

3. **Handlers**
   - Change handler signatures from `echo.Context` to `rweb.Context`
   - Update parameter access methods
   - Migrate response methods

4. **Middleware**
   - Update middleware signatures
   - Use `rweb.RequestInfo` as the first middleware for complete request logging
   - Ensure all middleware calls `ctx.Next()`
   - Migrate authentication to use context data storage

5. **Routing**
   - Update route definitions (case-sensitive methods in RWeb)
   - Migrate route groups
   - Update static file serving

6. **Authentication**
   - Implement auth middleware using `ctx.Set()`/`ctx.Get()`
   - Update protected routes to check context data
   - Implement proper logout to clear context data

7. **Testing**
   - Update test helpers for RWeb context
   - Test authentication flows thoroughly
   - Verify static file serving
   - Test SSE endpoints if used

## Common Pitfalls to Avoid

1. **Forgetting to call `ctx.Next()` in middleware** - This will stop the request chain
2. **Using wrong parameter methods** - Use `PathParam()` for route params, not `Param()`
3. **Not using `:8080` format for addresses** - Important for Docker compatibility
4. **Assuming automatic JSON binding** - RWeb requires manual unmarshaling
5. **Not checking context data existence** - Always use `ctx.Has()` before `ctx.Get()`
6. **Forgetting to defer file.Close()** in upload handlers

## Performance Considerations

1. RWeb uses a radix tree router which provides O(log n) route matching
2. Context data storage is lightweight - only initialized when used
3. Use buffered channels for SSE to prevent blocking
4. Consider connection pooling for database operations
5. Use `element.ForEach` for efficient list rendering

## Testing Strategy

RWeb provides built-in testing capabilities without requiring httptest. Follow these patterns:

### Basic Handler Testing
```go
func TestHandler(t *testing.T) {
    s := rweb.NewServer()
    
    s.Get("/hello", func(ctx rweb.Context) error {
        return ctx.WriteString("Hello, World!")
    })
    
    // Use Request method for synchronous testing
    response := s.Request("GET", "/hello", nil, nil)
    assert.Equal(t, response.Status(), 200)
    assert.Equal(t, string(response.Body()), "Hello, World!")
}
```

### Integration Testing with Running Server
```go
func TestIntegration(t *testing.T) {
    readyChan := make(chan struct{}, 1)
    
    s := rweb.NewServer(rweb.ServerOptions{
        Verbose: true,
        ReadyChan: readyChan,
        Address: "localhost:", // Let OS assign port
    })
    
    s.Get("/", func(ctx rweb.Context) error {
        return ctx.WriteString("Home")
    })
    
    go func() {
        defer syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
        
        <-readyChan // Wait for server to be ready
        
        // Get assigned port
        port := s.GetListenPort()
        
        // Make HTTP request
        resp, err := http.Get(fmt.Sprintf("http://localhost:%s/", port))
        assert.Nil(t, err)
        assert.Equal(t, resp.Status, "200 OK")
        
        body, _ := io.ReadAll(resp.Body)
        resp.Body.Close()
        assert.Equal(t, string(body), "Home")
    }()
    
    _ = s.Run()
}
```

### Testing with Authentication
```go
func TestAuthenticatedRoute(t *testing.T) {
    s := rweb.NewServer()
    
    // Auth middleware
    s.Use(func(ctx rweb.Context) error {
        token := ctx.Request().Header("Authorization")
        if token == "Bearer valid-token" {
            ctx.Set("isLoggedIn", true)
            ctx.Set("userId", "123")
        }
        return ctx.Next()
    })
    
    s.Get("/profile", func(ctx rweb.Context) error {
        if !ctx.Has("isLoggedIn") || !ctx.Get("isLoggedIn").(bool) {
            return ctx.Status(401).WriteString("Unauthorized")
        }
        userId := ctx.Get("userId").(string)
        return ctx.WriteString("User: " + userId)
    })
    
    // Test without auth
    response := s.Request("GET", "/profile", nil, nil)
    assert.Equal(t, response.Status(), 401)
    assert.Equal(t, string(response.Body()), "Unauthorized")
    
    // Test with auth
    headers := map[string]string{"Authorization": "Bearer valid-token"}
    response = s.Request("GET", "/profile", headers, nil)
    assert.Equal(t, response.Status(), 200)
    assert.Equal(t, string(response.Body()), "User: 123")
}
```

### Testing POST Requests
```go
func TestPostRequest(t *testing.T) {
    readyChan := make(chan struct{}, 1)
    
    s := rweb.NewServer(rweb.ServerOptions{
        Verbose: true,
        ReadyChan: readyChan,
        Address: "localhost:",
    })
    
    s.Post("/form", func(ctx rweb.Context) error {
        name := ctx.Request().FormValue("name")
        return ctx.WriteString("Hello, " + name)
    })
    
    go func() {
        defer syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
        
        <-readyChan
        
        // Send form data
        formData := bytes.NewReader([]byte("name=John&age=30"))
        resp, err := http.Post(
            fmt.Sprintf("http://localhost:%s/form", s.GetListenPort()),
            "application/x-www-form-urlencoded",
            formData,
        )
        assert.Nil(t, err)
        assert.Equal(t, resp.Status, "200 OK")
        
        body, _ := io.ReadAll(resp.Body)
        resp.Body.Close()
        assert.Equal(t, string(body), "Hello, John")
    }()
    
    _ = s.Run()
}
```

### Testing JSON APIs
```go
func TestJSONAPI(t *testing.T) {
    s := rweb.NewServer()
    
    type User struct {
        ID   string `json:"id"`
        Name string `json:"name"`
    }
    
    s.Post("/users", func(ctx rweb.Context) error {
        body, err := io.ReadAll(ctx.Request().Body())
        if err != nil {
            return err
        }
        
        var user User
        err = json.Unmarshal(body, &user)
        if err != nil {
            return ctx.Status(400).WriteJSON(map[string]string{
                "error": "Invalid JSON",
            })
        }
        
        user.ID = "123"
        return ctx.Status(201).WriteJSON(user)
    })
    
    // Test JSON request
    userJSON := []byte(`{"name": "Alice"}`)
    response := s.Request("POST", "/users", nil, userJSON)
    
    assert.Equal(t, response.Status(), 201)
    
    var result User
    err := json.Unmarshal(response.Body(), &result)
    assert.Nil(t, err)
    assert.Equal(t, result.ID, "123")
    assert.Equal(t, result.Name, "Alice")
}
```

### Testing Groups and Middleware
```go
func TestGroups(t *testing.T) {
    s := rweb.NewServer()
    
    // Public routes
    s.Get("/", func(ctx rweb.Context) error {
        return ctx.WriteString("Public")
    })
    
    // Admin group with middleware
    admin := s.Group("/admin", func(ctx rweb.Context) error {
        if ctx.Request().Header("X-Admin") != "true" {
            return ctx.Status(403).WriteString("Forbidden")
        }
        return ctx.Next()
    })
    
    admin.Get("/users", func(ctx rweb.Context) error {
        return ctx.WriteString("Admin Users")
    })
    
    // Test public route
    response := s.Request("GET", "/", nil, nil)
    assert.Equal(t, response.Status(), 200)
    assert.Equal(t, string(response.Body()), "Public")
    
    // Test admin route without header
    response = s.Request("GET", "/admin/users", nil, nil)
    assert.Equal(t, response.Status(), 403)
    
    // Test admin route with header
    headers := map[string]string{"X-Admin": "true"}
    response = s.Request("GET", "/admin/users", headers, nil)
    assert.Equal(t, response.Status(), 200)
    assert.Equal(t, string(response.Body()), "Admin Users")
}
```

### Key Testing Tips

1. **Use `s.Request()` for synchronous testing** - No need for httptest
2. **Use ReadyChan for integration tests** - Ensures server is ready before testing
3. **Use `s.GetListenPort()` for dynamic ports** - Avoids port conflicts
4. **Test middleware behavior** - Verify context data is set correctly
5. **Always close response bodies** - Prevent resource leaks
6. **Use `syscall.Kill()` to stop test servers** - Clean shutdown in tests

## Deployment Notes

1. Ensure all environment variables are updated
2. Update Docker configurations if needed
3. Review and update reverse proxy configurations
4. Monitor performance metrics after migration
5. Have rollback plan ready

## Church Application Specific Migration Plan

### 1. Custom Context Migration

The Church app uses a custom context that extends Echo's context. This needs to be reimplemented for RWeb:

**Current Echo Custom Context (`context/custom_context.go`):**
```go
type CustomContext struct {
    echo.Context
    Admin bool
    Session session.Session
}
```

**New RWeb Implementation:**
```go
// Since RWeb Context supports data storage, we'll use middleware to set these values
func UseCustomContext(ctx rweb.Context) error {
    // Load session from Redis
    sessId := getSessionIdFromCookie(ctx)
    if sessId != "" {
        sess, err := session.LoadSession(sessId)
        if err == nil {
            ctx.Set("session", sess)
            ctx.Set("isAdmin", sess.IsAdmin())
            ctx.Set("userId", sess.UserId)
            ctx.Set("username", sess.Username)
        }
    }
    return ctx.Next()
}

// Helper functions to access context data
func GetSession(ctx rweb.Context) session.Session {
    if ctx.Has("session") {
        return ctx.Get("session").(session.Session)
    }
    return session.Session{}
}

func IsAdmin(ctx rweb.Context) bool {
    if ctx.Has("isAdmin") {
        return ctx.Get("isAdmin").(bool)
    }
    return false
}
```

### 2. Router Migration (`router.go`)

**Current Echo Setup:**
```go
e := echo.New()
e.HideBanner = true
e.Static("/assets", "dist")
e.Static("/media", "sermons")
```

**New RWeb Setup:**
```go
s := rweb.NewServer(rweb.ServerOptions{
    Address: ":" + config.Options.Server.Port,
    Verbose: config.Options.Debug,
    TLS: rweb.TLSCfg{
        UseTLS:   config.Options.Server.UseTLS,
        KeyFile:  config.Options.Server.TLSKeyFile,
        CertFile: config.Options.Server.TLSCertFile,
    },
})

// Static files
s.StaticFiles("/assets/", "dist", 1)
s.StaticFiles("/media/", "sermons", 1)
```

### 3. Route Groups Migration

**Current Echo Groups:**
```go
pgs := e.Group("pages")
art := e.Group("articles")
evt := e.Group("events")
pay := e.Group("payments")
ser := e.Group("sermons")
ad := e.Group(config.AdminPrefix)
ad.Use(authctlr.UseCustomContext)
ad.Use(authctlr.AdminGuard)
```

**New RWeb Groups:**
```go
// Public groups
pgs := s.Group("/pages")
art := s.Group("/articles")
evt := s.Group("/events")
pay := s.Group("/payments")
ser := s.Group("/sermons")

// Admin group with middleware
ad := s.Group(config.AdminPrefix, UseCustomContext, AdminGuard)
```

### 4. Handler Signature Updates

All handlers need to be updated from:
```go
func HandlerName(c echo.Context) error
```

To:
```go
func HandlerName(ctx rweb.Context) error
```

### 5. Parameter Access Migration

**Path Parameters:**
```go
// Echo
c.Param("id")
c.Param("slug")

// RWeb
ctx.Request().PathParam("id")
ctx.Request().PathParam("slug")
```

**Query Parameters:**
```go
// Echo
c.QueryParam("offset")
c.QueryParam("limit")

// RWeb
ctx.Request().QueryParam("offset")
ctx.Request().QueryParam("limit")
```

**Form Values:**
```go
// Echo
c.FormValue("field_name")

// RWeb
ctx.Request().FormValue("field_name")
```

### 6. Response Methods Migration

**HTML Response:**
```go
// Echo
c.HTMLBlob(200, bytes)

// RWeb
ctx.WriteHTML(string(bytes))
```

**JSON Response:**
```go
// Echo
c.JSON(http.StatusOK, data)

// RWeb
ctx.WriteJSON(data)
// or with custom status
ctx.Status(201).WriteJSON(data)
```

**String Response:**
```go
// Echo
c.String(http.StatusOK, "text")

// RWeb
ctx.WriteString("text")
```

**File Streaming:**
```go
// Echo
c.Stream(http.StatusOK, "content-type", reader)

// RWeb
ctx.Response().Header().Set("Content-Type", "content-type")
io.Copy(ctx.Response(), reader)
```

### 7. Custom Redirect Function

The app has a custom redirect function that needs updating:
```go
// Current
func Redirect(c echo.Context, msg string, destination string, opts ...interface{}) error {
    // Set flash message in cookie
    return c.Redirect(code, destination)
}

// New RWeb version
func Redirect(ctx rweb.Context, msg string, destination string, opts ...interface{}) error {
    // Set flash message in cookie
    ctx.Response().Header().Set("Location", destination)
    ctx.Response().WriteHeader(code)
    return nil
}
```

### 8. File Download Handlers

Update SendFile and SendAudioFile functions:
```go
// Current Echo version
func SendAudioFile(c echo.Context, filePath, fileName string) error {
    c.Response().Header().Set("Content-Type", "audio/mpeg")
    c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, fileName))
    return c.Stream(http.StatusOK, "audio/mpeg", file)
}

// New RWeb version
func SendAudioFile(ctx rweb.Context, filePath, fileName string) error {
    ctx.Response().Header().Set("Content-Type", "audio/mpeg")
    ctx.Response().Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, fileName))
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer file.Close()
    _, err = io.Copy(ctx.Response(), file)
    return err
}
```

### 9. Session and Authentication Middleware

**Current AdminGuard:**
```go
func AdminGuard(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        cc := c.(*CustomContext)
        if !cc.Admin {
            return Redirect(cc, "warning", "/login", "msg", "Admin access required")
        }
        return next(c)
    }
}
```

**New RWeb AdminGuard:**
```go
func AdminGuard(ctx rweb.Context) error {
    if !IsAdmin(ctx) {
        return Redirect(ctx, "warning", "/login", "msg", "Admin access required")
    }
    return ctx.Next()
}
```

### 10. File Upload Migration

For sermon uploads and other file handling:
```go
// Echo
file, err := c.FormFile("file")
src, err := file.Open()

// RWeb
file, header, err := ctx.Request().GetFormFile("file")
// file is already opened
defer file.Close()
```

### 11. Migration Order

1. **Phase 1: Core Infrastructure**
   - Create new context helpers
   - Update router.go with RWeb server
   - Migrate middleware (UseCustomContext, AdminGuard)
   - Update redirect and flash message functions

2. **Phase 2: Controllers (by priority)**
   - auth_controller.go (authentication is critical)
   - page_controller.go (core CMS functionality)
   - article_controller.go
   - sermon_controller.go
   - event_controller.go
   - menu_controller.go
   - user_controller.go
   - payment_controller.go

3. **Phase 3: Resource Modules**
   - Update all module_*.go files in resource packages
   - Update presenters that generate responses

4. **Phase 4: API Endpoints**
   - /api/v1/sermons
   - Calendar endpoints
   - Debug endpoints

5. **Phase 5: Testing and Cleanup**
   - Update tests for new context
   - Remove Echo dependencies
   - Performance testing

### 12. Testing Strategy

Create integration tests for critical paths:
1. Authentication flow (login/logout)
2. Admin access control
3. Page rendering with modules
4. Sermon file uploads and downloads
5. Session persistence
6. Flash messages

### 13. Rollback Plan

1. Keep Echo implementation in a separate branch
2. Use feature flags to switch between Echo/RWeb during testing
3. Deploy to staging environment first
4. Monitor error logs and performance metrics
5. Have database backups before deployment

### 14. Special Considerations

1. **Redis Sessions**: Ensure session loading/saving works identically
2. **File Paths**: Verify media file serving works with new static file handler
3. **Flash Messages**: Test cookie-based flash messages thoroughly
4. **Module System**: Ensure JSONB module data rendering is unaffected
5. **TLS**: Test HTTPS configuration in production environment

This comprehensive plan should guide a smooth migration from Echo to RWeb while maintaining all functionality of the Church CMS application.