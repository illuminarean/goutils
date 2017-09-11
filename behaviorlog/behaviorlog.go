package behaviorlog

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pangpanglabs/goutils/kafka"
	"github.com/sirupsen/logrus"
)

type LogContext struct {
	Producer *kafka.Producer
	logger   *logrus.Logger

	Service       string                 `json:"service,omitempty"`
	Timestamp     time.Time              `json:"timestamp,omitempty"`
	RequestID     string                 `json:"request_id,omitempty"`
	RemoteIP      string                 `json:"remote_ip,omitempty"`
	Host          string                 `json:"host,omitempty"`
	Uri           string                 `json:"uri,omitempty"`
	Method        string                 `json:"method,omitempty"`
	Path          string                 `json:"path,omitempty"`
	Referer       string                 `json:"referer,omitempty"`
	UserAgent     string                 `json:"user_agent,omitempty"`
	Status        int                    `json:"status,omitempty"`
	Latency       time.Duration          `json:"latency,omitempty"`
	RequestLength int64                  `json:"request_length,omitempty"`
	BytesSent     int64                  `json:"bytes_sent,omitempty"`
	Params        interface{}            `json:"params,omitempty"`
	Controller    string                 `json:"controller,omitempty"`
	Action        string                 `json:"action,omitempty"`
	Body          string                 `json:"body,omitempty"`
	BizAttr       map[string]interface{} `json:"bizAttr,omitempty"`
	Err           error                  `json:"error,omitempty"`
}

const (
	HeaderXRequestID    = "X-Request-ID"
	HeaderXForwardedFor = "X-Forwarded-For"
	HeaderXRealIP       = "X-Real-IP"
	HeaderContentLength = "Content-Length"
)

var logger = logrus.New()

func init() {
	logger.Formatter = &logrus.JSONFormatter{}
}

func New(serviceName string, req *http.Request, options ...func(*LogContext)) *LogContext {
	realIP := req.RemoteAddr
	if ip := req.Header.Get(HeaderXForwardedFor); ip != "" {
		realIP = strings.Split(ip, ", ")[0]
	} else if ip := req.Header.Get(HeaderXRealIP); ip != "" {
		realIP = ip
	} else {
		realIP, _, _ = net.SplitHostPort(realIP)
	}

	path := req.URL.Path
	if path == "" {
		path = "/"
	}

	requestLength, _ := strconv.ParseInt(req.Header.Get(HeaderContentLength), 10, 64)

	params := map[string]string{}
	for k, v := range req.URL.Query() {
		params[k] = v[0]
	}

	c := &LogContext{
		// Producer: producer,
		logger: logger,

		Service:       serviceName,
		Timestamp:     time.Now(),
		RequestID:     req.Header.Get(HeaderXRequestID),
		RemoteIP:      realIP,
		Host:          req.Host,
		Uri:           req.RequestURI,
		Method:        req.Method,
		Path:          path,
		Params:        params,
		Referer:       req.Referer(),
		UserAgent:     req.UserAgent(),
		RequestLength: requestLength,
		// Controller: controller,
		// Action:     action,
		BizAttr: map[string]interface{}{},
	}

	for _, o := range options {
		if o != nil {
			o(c)
		}
	}

	return c
}

func KafkaProducer(p *kafka.Producer) func(*LogContext) {
	return func(l *LogContext) {
		l.Producer = p
	}

}
func (c *LogContext) Clone() *LogContext {
	return &LogContext{
		Producer:      c.Producer,
		logger:        c.logger,
		Service:       c.Service,
		Timestamp:     time.Now(),
		RequestID:     c.RequestID,
		RemoteIP:      c.RemoteIP,
		Host:          c.Host,
		Uri:           c.Uri,
		Method:        c.Method,
		Path:          c.Path,
		Referer:       c.Referer,
		UserAgent:     c.UserAgent,
		Status:        c.Status,
		Latency:       c.Latency,
		RequestLength: c.RequestLength,
		BytesSent:     c.BytesSent,
		Params:        c.Params,
		Controller:    c.Controller,
		Action:        c.Action,
		Body:          c.Body,
		BizAttr:       map[string]interface{}{},
	}
}

func (c *LogContext) WithBizAttr(key string, value interface{}) *LogContext {
	c.BizAttr[key] = value
	return c
}
func (c *LogContext) WithControllerAndAction(controller, action string) *LogContext {
	c.Controller = controller
	c.Action = action
	return c
}
func (c *LogContext) WithError(err error) *LogContext {
	c.Err = err
	return c
}
func (c *LogContext) WithBizAttrs(attrs map[string]interface{}) *LogContext {
	for k, v := range attrs {
		c.BizAttr[k] = v
	}
	return c
}
func (c *LogContext) WithCallURLInfo(method, uri string, param interface{}, responseStatus int) *LogContext {
	c.Method = method
	c.Uri = uri
	c.Params = param
	c.Status = responseStatus
	if url, err := url.Parse(uri); err == nil {
		c.Path = url.Path
		c.Host = url.Host
	}

	return c
}
func (c *LogContext) Log(action string) {
	c.Action = action
	c.Write()
}
func (c *LogContext) Write() {
	c.Latency = time.Now().Sub(c.Timestamp)
	if c.Producer != nil {
		c.Producer.Send(c)
	}
	logEntry := c.logger.WithFields(logrus.Fields{
		"service":        c.Service,
		"timestamp":      c.Timestamp.UTC().Format(time.RFC3339),
		"request_id":     c.RequestID,
		"remote_ip":      c.RemoteIP,
		"host":           c.Host,
		"uri":            c.Uri,
		"method":         c.Method,
		"path":           c.Path,
		"referer":        c.Referer,
		"user_agent":     c.UserAgent,
		"status":         c.Status,
		"latency":        c.Latency,
		"request_length": c.RequestLength,
		"bytes_sent":     c.BytesSent,
		"params":         c.Params,
		"controller":     c.Controller,
		"action":         c.Action,
		"body":           c.Body,
		"bizAttr":        c.BizAttr,
	})
	if c.Err != nil {
		logEntry = logEntry.WithError(c.Err)
	}
	logEntry.Info()
}
func NewNopContext() *LogContext {
	return &LogContext{
		logger:  logrus.New(),
		BizAttr: map[string]interface{}{},
	}
}