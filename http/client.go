package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"gx/ipfs/QmYiqbfRCkryYvJsxBopy77YEhxNZXTmq5Y2qiKyenc59C/go-ipfs-cmdkit"

	config "github.com/ipfs/go-ipfs/repo/config"
)

const (
	ApiUrlFormat = "http://%s%s/%s?%s"
	ApiPath      = "/api/v0" // TODO: make configurable
)

var OptionSkipMap = map[string]bool{
	"api": true,
}

// Client is the commands HTTP client interface.
type Client interface {
	Send(req cmds.Request) (cmds.Response, error)
}

type client struct {
	serverAddress string
	httpClient    *http.Client
}

func NewClient(address string) Client {
	return &client{
		serverAddress: address,
		httpClient:    http.DefaultClient,
	}
}

func (c *client) Send(req cmds.Request) (cmds.Response, error) {

	if req.Context() == nil {
		log.Warningf("no context set in request")
		if err := req.SetRootContext(context.TODO()); err != nil {
			return nil, err
		}
	}

	// save user-provided encoding
	previousUserProvidedEncoding, found, err := req.Option(cmdsutil.EncShort).String()
	if err != nil {
		return nil, err
	}

	// override with json to send to server
	req.SetOption(cmdsutil.EncShort, cmds.JSON)

	// stream channel output
	req.SetOption(cmdsutil.ChanOpt, "true")

	query, err := getQuery(req)
	if err != nil {
		return nil, err
	}

	var fileReader *MultiFileReader
	var reader io.Reader

	if req.Files() != nil {
		fileReader = NewMultiFileReader(req.Files(), true)
		reader = fileReader
	}

	path := strings.Join(req.Path(), "/")
	url := fmt.Sprintf(ApiUrlFormat, c.serverAddress, ApiPath, path, query)

	httpReq, err := http.NewRequest("POST", url, reader)
	if err != nil {
		return nil, err
	}

	// TODO extract string consts?
	if fileReader != nil {
		httpReq.Header.Set(contentTypeHeader, "multipart/form-data; boundary="+fileReader.Boundary())
	} else {
		httpReq.Header.Set(contentTypeHeader, applicationOctetStream)
	}
	httpReq.Header.Set(uaHeader, config.ApiVersion)

	httpReq.Cancel = req.Context().Done()
	httpReq.Close = true

	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	// using the overridden JSON encoding in request
	res, err := getResponse(httpRes, req)
	if err != nil {
		return nil, err
	}

	if found && len(previousUserProvidedEncoding) > 0 {
		// reset to user provided encoding after sending request
		// NB: if user has provided an encoding but it is the empty string,
		// still leave it as JSON.
		req.SetOption(cmdsutil.EncShort, previousUserProvidedEncoding)
	}

	return res, nil
}

func getQuery(req cmds.Request) (string, error) {
	query := url.Values{}
	for k, v := range req.Options() {
		if OptionSkipMap[k] {
			continue
		}
		str := fmt.Sprintf("%v", v)
		query.Set(k, str)
	}

	args := req.StringArguments()
	argDefs := req.Command().Arguments

	argDefIndex := 0

	for _, arg := range args {
		argDef := argDefs[argDefIndex]
		// skip ArgFiles
		for argDef.Type == cmdsutil.ArgFile {
			argDefIndex++
			argDef = argDefs[argDefIndex]
		}

		query.Add("arg", arg)

		if len(argDefs) > argDefIndex+1 {
			argDefIndex++
		}
	}

	return query.Encode(), nil
}
