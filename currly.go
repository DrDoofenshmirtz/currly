package currly

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

func ClientConnector(c *http.Client) Connector {
	return clientConnector{c}
}

func DefaultConnector() Connector {
	t := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	c := &http.Client{Transport: t}

	return ClientConnector(c)
}

func Builder(c Connector) DefineMethod {
	return curlTemplate{connector: c}
}

func PathArg(name, value string) Arg {
	return argFunc(func(ct *curlTemplate) error {
		for _, v := range ct.urlTemplate.path {
			if v.varName() == name && v.bindTo(value) {
				return nil
			}
		}

		return fmt.Errorf("failed to bind value '%v' to URL path parameter '%v'", value, name)
	})
}

func QueryArg(name, value string) Arg {
	return argFunc(func(ct *curlTemplate) error {
		for _, v := range ct.urlTemplate.query {
			if v.varName() == name && v.bindTo(value) {
				return nil
			}
		}

		return fmt.Errorf("failed to bind value '%v' to URL query parameter '%v'", value, name)
	})
}

func JSONBodyArg(body interface{}) Arg {
	once := sync.Once{}

	return argFunc(func(ct *curlTemplate) error {
		var bs []byte
		var err error

		once.Do(func() { bs, err = json.Marshal(body) })

		if err != nil {
			return err
		}

		if ct.header == nil {
			ct.header = make(http.Header)
		}

		ct.header.Set("Content-Type", "application/json; charset=utf-8")
		ct.body = ioutil.NopCloser(bytes.NewReader(bs))

		return nil
	})
}

func JSONStringExtractor() ResultExtractor {
	return ResultExtractorFunc(func(r *http.Response) (interface{}, error) {
		bs, err := ioutil.ReadAll(r.Body)

		if err != nil {
			return nil, err
		}

		buf := new(bytes.Buffer)
		err = json.Indent(buf, bs, "", "  ")

		if err != nil {
			return nil, err
		}

		return string(buf.Bytes()), nil
	})
}

func PlainStringExtractor() ResultExtractor {
	return ResultExtractorFunc(func(r *http.Response) (interface{}, error) {
		bs, err := ioutil.ReadAll(r.Body)

		if err != nil {
			return nil, err
		}

		return string(bs), nil
	})
}

func BytesExtractor() ResultExtractor {
	return ResultExtractorFunc(func(r *http.Response) (interface{}, error) {
		bs, err := ioutil.ReadAll(r.Body)

		if err != nil {
			return nil, err
		}

		return bs, nil
	})
}

type Connector interface {
	Send(r *http.Request) (*http.Response, error)
}

type DefineMethod interface {
	Method(method string) DefineScheme
	GET() DefineScheme
	POST() DefineScheme
}

type DefineScheme interface {
	HTTP() DefineHost
	HTTPS() DefineHost
}

type DefineHost interface {
	Host(host string) DefinePort
	Localhost() DefinePort
}

type DefinePort interface {
	pathPart
	queryPart
	headerPart
	credentialsPart
	resultExtractorPart
	curlFuncPart

	Port(port uint) BuildPath
}

type BuildPath interface {
	pathPart
	queryPart
	headerPart
	credentialsPart
	resultExtractorPart
	curlFuncPart
}

type BuildQuery interface {
	queryPart
	headerPart
	credentialsPart
	resultExtractorPart
	curlFuncPart
}

type SetCredentials interface {
	credentialsPart
	resultExtractorPart
	curlFuncPart
}

type SetResultExtractor interface {
	resultExtractorPart
	curlFuncPart
}

type BuildCurl interface {
	curlFuncPart
}

type CurlFunc func(args ...Arg) (int, interface{}, error)

type Arg interface {
	applyTo(ct *curlTemplate) error
}

type ResultExtractor interface {
	Result(r *http.Response) (interface{}, error)
}

type ResultExtractorFunc func(r *http.Response) (interface{}, error)

type clientConnector struct {
	*http.Client
}

type pathPart interface {
	PathSegment(name string) BuildPath
	PathParam(name string) BuildPath
}

type queryPart interface {
	QuerySegment(name, value string) BuildQuery
	QueryParam(name string) BuildQuery
}

type headerPart interface {
	Header(header http.Header) SetCredentials
}

type credentialsPart interface {
	Credentials(username, password string) SetResultExtractor
}

type resultExtractorPart interface {
	ResultExtractor(r ResultExtractor) BuildCurl
}

type curlFuncPart interface {
	Build() (CurlFunc, error)
}

type variable interface {
	fmt.Stringer
	varName() string
	bindTo(value string) bool
	copy() variable
}

type curlTemplate struct {
	connector       Connector
	method          string
	urlTemplate     urlTemplate
	header          http.Header
	credentials     credentials
	body            io.ReadCloser
	resultExtractor ResultExtractor
	error           error
}

type urlTemplate struct {
	scheme string
	host   string
	port   uint
	path   []variable
	query  []variable
}

type pathSegment struct {
	name string
}

type pathParam struct {
	name  string
	value string
}

type querySegment struct {
	name  string
	value string
}

type queryParam struct {
	name  string
	value string
}

type credentials struct {
	username string
	password string
}

type argFunc func(ct *curlTemplate) error

func (cc clientConnector) Send(r *http.Request) (*http.Response, error) {
	return cc.Do(r)
}

func (ct curlTemplate) Method(method string) DefineScheme {
	ct.method = method

	return ct
}

func (ct curlTemplate) GET() DefineScheme {
	return ct.Method(http.MethodGet)
}

func (ct curlTemplate) POST() DefineScheme {
	return ct.Method(http.MethodPost)
}

func (ct curlTemplate) HTTP() DefineHost {
	ct.urlTemplate.scheme = "http"

	return ct
}

func (ct curlTemplate) HTTPS() DefineHost {
	ct.urlTemplate.scheme = "https"

	return ct
}

func (ct curlTemplate) Host(host string) DefinePort {
	ct.urlTemplate.host = host

	return ct
}

func (ct curlTemplate) Localhost() DefinePort {
	return ct.Host("localhost")
}

func (ct curlTemplate) Port(port uint) BuildPath {
	ct.urlTemplate.port = port

	return ct
}

func (ct curlTemplate) PathSegment(name string) BuildPath {
	ct.urlTemplate.path = append(ct.urlTemplate.path, &pathSegment{name})

	return ct
}

func (ct curlTemplate) PathParam(name string) BuildPath {
	ct.urlTemplate.path = append(ct.urlTemplate.path, &pathParam{name: name})

	return ct
}

func (ct curlTemplate) QuerySegment(name, value string) BuildQuery {
	ct.urlTemplate.query = append(ct.urlTemplate.query, &querySegment{name, value})

	return ct
}

func (ct curlTemplate) QueryParam(name string) BuildQuery {
	ct.urlTemplate.query = append(ct.urlTemplate.query, &queryParam{name: name})

	return ct
}

func (ct curlTemplate) Header(header http.Header) SetCredentials {
	ct.header = header

	return ct
}

func (ct curlTemplate) Credentials(username, password string) SetResultExtractor {
	ct.credentials = credentials{username, password}

	return ct
}

func (ct curlTemplate) ResultExtractor(r ResultExtractor) BuildCurl {
	ct.resultExtractor = r

	return ct
}

func (ct curlTemplate) Build() (CurlFunc, error) {
	if ct.error != nil {
		return nil, ct.error
	}

	return CurlFunc(func(args ...Arg) (int, interface{}, error) {
		ct = complete(ct, args)

		if ct.error != nil {
			return 0, nil, ct.error
		}

		req, err := createRequest(ct)

		if err != nil {
			return 0, nil, err
		}

		resp, err := ct.connector.Send(req)

		if err != nil {
			return 0, nil, err
		}

		defer resp.Body.Close()

		ret, err := ct.resultExtractor.Result(resp)

		if err != nil {
			return resp.StatusCode, nil, err
		}

		return resp.StatusCode, ret, nil
	}), nil
}

func complete(ct curlTemplate, args []Arg) curlTemplate {
	ct = copyCurlTemplate(ct)

	if ct.resultExtractor == nil {
		ct.resultExtractor = JSONStringExtractor()
	}

	for _, a := range args {
		err := a.applyTo(&ct)

		if err != nil {
			ct.error = err

			return ct
		}
	}

	return ct
}

func copyCurlTemplate(ct curlTemplate) curlTemplate {
	ct.urlTemplate = copyURLTemplate(ct.urlTemplate)
	ct.header = copyHeader(ct.header)

	return ct
}

func copyHeader(h http.Header) http.Header {
	hCopy := make(http.Header, len(h))

	for k, v := range h {
		vCopy := make([]string, len(v))

		copy(vCopy, v)
		hCopy[k] = vCopy
	}

	return hCopy
}

func copyURLTemplate(ut urlTemplate) urlTemplate {
	ut.path = copyVariables(ut.path)
	ut.query = copyVariables(ut.query)

	return ut
}

func copyVariables(vs []variable) []variable {
	vsCopy := make([]variable, len(vs))

	for i, v := range vs {
		vsCopy[i] = v.copy()
	}

	return vsCopy
}

var emptyCredentials credentials

func createRequest(ct curlTemplate) (*http.Request, error) {
	r, err := http.NewRequest(ct.method, urlString(ct.urlTemplate), ct.body)

	if err != nil {
		return nil, err
	}

	if ct.header != nil {
		for k, v := range ct.header {
			r.Header[k] = v
		}
	}

	if ct.credentials != emptyCredentials {
		r.SetBasicAuth(ct.credentials.username, ct.credentials.password)
	}

	return r, nil
}

func urlString(ut urlTemplate) string {
	url := ut.scheme + "://" + ut.host

	if ut.port > 0 {
		url = url + ":" + strconv.FormatUint(uint64(ut.port), 10)
	}

	path := ""

	for _, v := range ut.path {
		s := v.String()

		if len(s) > 0 {
			if len(path) > 0 {
				path = path + "/"
			}

			path = path + s
		}
	}

	if len(path) > 0 {
		url = url + "/" + path
	}

	query := ""

	for _, v := range ut.query {
		s := v.String()

		if len(s) > 0 {
			if len(query) > 0 {
				query = query + "&"
			}

			query = query + s
		}
	}

	if len(query) > 0 {
		url = url + "?" + query
	}

	return url
}

func (ps *pathSegment) varName() string {
	return ps.name
}

func (ps *pathSegment) bindTo(value string) bool {
	return false
}

func (ps *pathSegment) copy() variable {
	return ps
}

func (ps *pathSegment) String() string {
	return ps.name
}

func (pp *pathParam) varName() string {
	return pp.name
}

func (pp *pathParam) bindTo(value string) bool {
	pp.value = value

	return true
}

func (pp *pathParam) copy() variable {
	copy := *pp

	return &copy
}

func (pp *pathParam) String() string {
	if len(pp.value) == 0 {
		return ""
	}

	return url.PathEscape(pp.value)
}

func (qs *querySegment) varName() string {
	return qs.name
}

func (qs *querySegment) bindTo(value string) bool {
	return false
}

func (qs *querySegment) copy() variable {
	return qs
}

func (qs *querySegment) String() string {
	return url.QueryEscape(qs.name) + "=" + url.QueryEscape(qs.value)
}

func (qp *queryParam) varName() string {
	return qp.name
}

func (qp *queryParam) bindTo(value string) bool {
	qp.value = value

	return true
}

func (qp *queryParam) copy() variable {
	copy := *qp

	return &copy
}

func (qp *queryParam) String() string {
	if len(qp.value) == 0 {
		return ""
	}

	return url.QueryEscape(qp.name) + "=" + url.QueryEscape(qp.value)
}

func (f argFunc) applyTo(ct *curlTemplate) error {
	return f(ct)
}

func (f ResultExtractorFunc) Result(r *http.Response) (interface{}, error) {
	return f(r)
}
