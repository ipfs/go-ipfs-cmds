package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"

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
	Send(req *cmds.Request) (cmds.Response, error)
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

func (c *client) Send(req *cmds.Request) (cmds.Response, error) {
	if req.Context == nil {
		log.Warningf("no context set in request")
		req.Context = context.TODO()
	}

	// save user-provided encoding
	previousUserProvidedEncoding, found := req.Options[cmdkit.EncShort].(string)

	// override with json to send to server
	req.Options[cmdkit.EncShort] = cmds.JSON

	// stream channel output
	req.Options[cmdkit.ChanOpt] = true

	query, err := getQuery(req)
	if err != nil {
		return nil, err
	}

	var fileReader *MultiFileReader
	var reader io.Reader

	if req.Files != nil {
		fileReader = NewMultiFileReader(req.Files, true)
		reader = fileReader
	}

	path := strings.Join(req.Path, "/")
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

	httpReq.Cancel = req.Context.Done()
	httpReq.Close = true

	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	// using the overridden JSON encoding in request
	res, err := parseResponse(httpRes, req)
	if err != nil {
		return nil, err
	}

	if found && len(previousUserProvidedEncoding) > 0 {
		// reset to user provided encoding after sending request
		// NB: if user has provided an encoding but it is the empty string,
		// still leave it as JSON.
		req.Options[cmdkit.EncShort] = previousUserProvidedEncoding
	}

	return res, nil
}

func getQuery(req *cmds.Request) (string, error) {
	query := url.Values{}

	for k, v := range req.Options {
		if OptionSkipMap[k] {
			continue
		}
		str := fmt.Sprintf("%v", v)
		query.Set(k, str)
	}

	args := req.Arguments
	argDefs := req.Command.Arguments

	argDefIndex := 0

	for _, arg := range args {
		argDef := argDefs[argDefIndex]
		// skip ArgFiles
		for argDef.Type == cmdkit.ArgFile {
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
