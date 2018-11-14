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

func BodyArg(body map[string]interface{}) Arg {
	return argFunc(func(ct *curlTemplate) bool {
		ct.body = body

		return true
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
	curlFuncBuilder

	Port(port uint) BuildPathStep
}

type BuildPathStep interface {
	pathBuilder
	queryBuilder
	headerBuilder
	credentialsBuilder
	curlFuncBuilder
}

type BuildQueryStep interface {
	queryBuilder
	headerBuilder
	credentialsBuilder
	curlFuncBuilder
}

type BuildCurlFuncStep interface {
	curlFuncBuilder
}

type CurlFunc func(args ...Arg) (interface{}, error)

type Arg interface {
	applyTo(ct *curlTemplate) bool
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

type curlFuncBuilder interface {
	Build() (CurlFunc, error)
}

type variable interface {
	fmt.Stringer
	varName() string
	bindTo(value string) bool
}

type curlTemplate struct {
	connector   Connector
	method      string
	urlTemplate urlTemplate
	header      http.Header
	credentials *credentials
	body        map[string]interface{}
	error       error
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
	ct.credentials = &credentials{username, password}

	return ct
}

func (ct curlTemplate) Port(port uint) BuildPathStep {
	ct.urlTemplate.port = port

	return ct
}

func (ct curlTemplate) Build() (CurlFunc, error) {
	if ct.error != nil {
		return nil, ct.error
	}

	return CurlFunc(func(args ...Arg) (interface{}, error) {
		ct = complete(ct, args)

		if ct.error != nil {
			return nil, ct.error
		}

		req, err := createRequest(ct)

		if err != nil {
			return nil, err
		}

		resp, err := ct.connector.Send(req)

		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		bs, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			return nil, err
		}

		buf := new(bytes.Buffer)
		err = json.Indent(buf, bs, "", "  ")

		if err != nil {
			return nil, err
		}

		return string(buf.Bytes()), nil
	}), nil
}

func complete(ct curlTemplate, args []Arg) curlTemplate {
	ct = copyCurlTemplate(ct)

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

func createRequest(ct curlTemplate) (*http.Request, error) {
	body, err := requestBody(ct.body)

	if err != nil {
		return nil, err
	}

	var r *http.Request

	r, err = http.NewRequest(ct.method, urlString(ct.urlTemplate), body)

	if err != nil {
		return nil, err
	}

	if ct.header != nil {
		for k, v := range ct.header {
			r.Header[k] = v
		}
	}

	if ct.credentials != nil {
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

func requestBody(body map[string]interface{}) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}

	bs, err := json.Marshal(body)

	if err != nil {
		return nil, err
	}

	return bytes.NewReader(bs), nil
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
