package notify

import "github.com/tiechui1994/tool/util"

var (
	defaultURL     string
	defaultSubject string
	defaultFrom    EmailInfo
)

func DefaultURL(u string) {
	defaultURL = u
}
func DefaultSubject(subject string) {
	defaultSubject = subject
}
func DefaultFrom(from EmailInfo) {
	defaultFrom = from
}

type EmailType = string

const (
	TypePlain EmailType = "text/plain"
	TypeHtml  EmailType = "text/html"
)

type EmailInfo struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type EmailContent struct {
	Type  EmailType `json:"type"`
	Value string    `json:"value"`
}

type emailOption struct {
	url     string
	subject string
	cc      []*EmailInfo
	to      []*EmailInfo
	from    *EmailInfo
	content []*EmailContent
}

type Option interface {
	apply(opt *emailOption)
}

type funcOption struct {
	fun func(opt *emailOption)
}

func newFunc(fun func(opt *emailOption)) *funcOption {
	return &funcOption{fun: fun}
}

func (o *funcOption) apply(opt *emailOption) {
	o.fun(opt)
}

func WithURL(url string) Option {
	return newFunc(func(opt *emailOption) {
		opt.url = url
	})
}

func WithSubject(subject string) Option {
	return newFunc(func(opt *emailOption) {
		opt.subject = subject
	})
}

func WithFrom(from EmailInfo) Option {
	return newFunc(func(opt *emailOption) {
		opt.from = &from
	})
}

func WithCC(cc EmailInfo) Option {
	return newFunc(func(opt *emailOption) {
		opt.cc = append(opt.cc, &cc)
	})
}

func WithTo(to EmailInfo) Option {
	return newFunc(func(opt *emailOption) {
		opt.to = append(opt.to, &to)
	})
}

func WithContent(_type EmailType, content string) Option {
	return newFunc(func(opt *emailOption) {
		opt.content = append(opt.content, &EmailContent{
			Type:  _type,
			Value: content,
		})
	})
}

func SendEmail(opts ...Option) error {
	option := &emailOption{
		url:     defaultURL,
		from:    &defaultFrom,
		subject: defaultSubject,
	}

	for _, opt := range opts {
		opt.apply(option)
	}

	body := map[string]interface{}{
		"to":      option.to,
		"cc":      option.cc,
		"from":    option.from,
		"subject": option.subject,
		"content": option.content,
	}

	_, err := util.POST(option.url, util.WithBody(body), util.WithRetry(2))
	return err
}
