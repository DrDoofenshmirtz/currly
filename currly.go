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

func Builder(c Connector) DefineMethodStep {
	return curlTemplate{connector: c}
}

func PathArg(name, value string) Arg {
	return argFunc(func(ct *curlTemplate) bool {
		for _, v := range ct.urlTemplate.path {
			if v.varName() == name && v.bindTo(value) {
				return true
			}
		}

		return false
	})
}

func QueryArg(name, value string) Arg {
	return argFunc(func(ct *curlTemplate) bool {
		for _, v := range ct.urlTemplate.query {
			if v.varName() == name && v.bindTo(value) {
				return true
			}
		}

		return false
	})
}

func JSONBodyArg(body interface{}) Arg {
	once := sync.Once{}

	return argFunc(func(ct *curlTemplate) bool {
		var bs []byte
		var err error

		once.Do(func() { bs, err = json.Marshal(body) })

		if err != nil {
			return false
		}

		if ct.header == nil {
			ct.header = make(http.Header)
		}

		ct.header.Set("Content-Type", "application/json; charset=utf-8")
		ct.body = ioutil.NopCloser(bytes.NewReader(bs))

		return true
	})
}

func JSONStringExtractor() ResultExtractor {
	return resultExtractorFunc(func(r *http.Response) (interface{}, error) {
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

type Connector interface {
	Send(r *http.Request) (*http.Response, error)
}

type DefineMethodStep interface {
	Method(method string) DefineSchemeStep
	GET() DefineSchemeStep
	POST() DefineSchemeStep
}

type DefineSchemeStep interface {
	HTTP() DefineHostStep
	HTTPS() DefineHostStep
}

type DefineHostStep interface {
	Host(host string) DefinePortStep
	Localhost() DefinePortStep
}

type DefinePortStep interface {
	pathBuilder
	queryBuilder
	headerBuilder
	credentialsBuilder
	resultExtractorBuilder
	curlFuncBuilder

	Port(port uint) BuildPathStep
}

type BuildPathStep interface {
	pathBuilder
	queryBuilder
	headerBuilder
	credentialsBuilder
	resultExtractorBuilder
	curlFuncBuilder
}

type BuildQueryStep interface {
	queryBuilder
	headerBuilder
	credentialsBuilder
	resultExtractorBuilder
	curlFuncBuilder
}

type BuildCurlFuncStep interface {
	curlFuncBuilder
}

type CurlFunc func(args ...Arg) (int, interface{}, error)

type Arg interface {
	applyTo(ct *curlTemplate) bool
}

type ResultExtractor interface {
	Result(r *http.Response) (interface{}, error)
}

type clientConnector struct {
	*http.Client
}

type pathBuilder interface {
	PathSegment(name string) BuildPathStep
	PathParam(name string) BuildPathStep
}

type queryBuilder interface {
	QuerySegment(name, value string) BuildQueryStep
	QueryParam(name string) BuildQueryStep
}

type headerBuilder interface {
	Header(header http.Header) BuildCurlFuncStep
}

type credentialsBuilder interface {
	Credentials(username, password string) BuildCurlFuncStep
}

type resultExtractorBuilder interface {
	ResultExtractor(r ResultExtractor) BuildCurlFuncStep
}

type curlFuncBuilder interface {
	Build() (CurlFunc, error)
}

type variable interface {
	fmt.Stringer
	varName() string
	bindTo(value string) bool
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

type argFunc func(ct *curlTemplate) bool

type resultExtractorFunc func(r *http.Response) (interface{}, error)

func (cc clientConnector) Send(r *http.Request) (*http.Response, error) {
	return cc.Do(r)
}

func (ct curlTemplate) Method(method string) DefineSchemeStep {
	ct.method = method

	return ct
}

func (ct curlTemplate) GET() DefineSchemeStep {
	return ct.Method(http.MethodGet)
}

func (ct curlTemplate) POST() DefineSchemeStep {
	return ct.Method(http.MethodPost)
}

func (ct curlTemplate) HTTP() DefineHostStep {
	ct.urlTemplate.scheme = "http"

	return ct
}

func (ct curlTemplate) HTTPS() DefineHostStep {
	ct.urlTemplate.scheme = "https"

	return ct
}

func (ct curlTemplate) Host(host string) DefinePortStep {
	ct.urlTemplate.host = host

	return ct
}

func (ct curlTemplate) Localhost() DefinePortStep {
	return ct.Host("localhost")
}

func (ct curlTemplate) Port(port uint) BuildPathStep {
	ct.urlTemplate.port = port

	return ct
}

func (ct curlTemplate) PathSegment(name string) BuildPathStep {
	ct.urlTemplate.path = append(ct.urlTemplate.path, &pathSegment{name})

	return ct
}

func (ct curlTemplate) PathParam(name string) BuildPathStep {
	ct.urlTemplate.path = append(ct.urlTemplate.path, &pathParam{name: name})

	return ct
}

func (ct curlTemplate) QuerySegment(name, value string) BuildQueryStep {
	ct.urlTemplate.query = append(ct.urlTemplate.query, &querySegment{name, value})

	return ct
}

func (ct curlTemplate) QueryParam(name string) BuildQueryStep {
	ct.urlTemplate.query = append(ct.urlTemplate.query, &queryParam{name: name})

	return ct
}

func (ct curlTemplate) Header(header http.Header) BuildCurlFuncStep {
	ct.header = header

	return ct
}

func (ct curlTemplate) Credentials(username, password string) BuildCurlFuncStep {
	ct.credentials = credentials{username, password}

	return ct
}

func (ct curlTemplate) ResultExtractor(r ResultExtractor) BuildCurlFuncStep {
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
		a.applyTo(&ct)
	}

	return ct
}

func copyCurlTemplate(ct curlTemplate) curlTemplate {
	copy := ct
	copy.urlTemplate = copyURLTemplate(copy.urlTemplate)

	return copy
}

func copyURLTemplate(ut urlTemplate) urlTemplate {
	copy := ut

	return copy
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

func (qp *queryParam) String() string {
	if len(qp.value) == 0 {
		return ""
	}

	return url.QueryEscape(qp.name) + "=" + url.QueryEscape(qp.value)
}

func (f argFunc) applyTo(ct *curlTemplate) bool {
	return f(ct)
}

func (f resultExtractorFunc) Result(r *http.Response) (interface{}, error) {
	return f(r)
}
