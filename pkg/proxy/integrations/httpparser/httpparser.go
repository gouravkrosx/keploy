package httpparser

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/cfssl/log"
	"go.keploy.io/server/pkg"
	"go.keploy.io/server/pkg/hooks"
	"go.keploy.io/server/pkg/models"
	"go.keploy.io/server/pkg/proxy/util"
	"go.keploy.io/server/utils"
	"go.uber.org/zap"
)

type HttpParser struct {
	logger *zap.Logger
	hooks  *hooks.Hook
}

// ProcessOutgoing implements proxy.DepInterface.
func (http *HttpParser) ProcessOutgoing(request []byte, clientConn, destConn net.Conn, ctx context.Context) {
	switch models.GetMode() {
	case models.MODE_RECORD:
		err := encodeOutgoingHttp(request, clientConn, destConn, http.logger, http.hooks, ctx)
		if err != nil {
			http.logger.Error("failed to encode the http message into the yaml", zap.Error(err))
			return
		}

	case models.MODE_TEST:
		decodeOutgoingHttp(request, clientConn, destConn, http.hooks, http.logger)
	default:
		http.logger.Info("Invalid mode detected while intercepting outgoing http call", zap.Any("mode", models.GetMode()))
	}

}

func NewHttpParser(logger *zap.Logger, h *hooks.Hook) *HttpParser {
	return &HttpParser{
		logger: logger,
		hooks:  h,
	}
}

// IsOutgoingHTTP function determines if the outgoing network call is HTTP by comparing the
// message format with that of an HTTP text message.
func (h *HttpParser) OutgoingType(buffer []byte) bool {
	return bytes.HasPrefix(buffer[:], []byte("HTTP/")) ||
		bytes.HasPrefix(buffer[:], []byte("GET ")) ||
		bytes.HasPrefix(buffer[:], []byte("POST ")) ||
		bytes.HasPrefix(buffer[:], []byte("PUT ")) ||
		bytes.HasPrefix(buffer[:], []byte("PATCH ")) ||
		bytes.HasPrefix(buffer[:], []byte("DELETE ")) ||
		bytes.HasPrefix(buffer[:], []byte("OPTIONS ")) ||
		bytes.HasPrefix(buffer[:], []byte("HEAD "))
}

func isJSON(body []byte) bool {
	var js interface{}
	return json.Unmarshal(body, &js) == nil
}

func mapsHaveSameKeys(map1 map[string]string, map2 map[string][]string) bool {
	if len(map1) != len(map2) {
		return false
	}

	for key := range map1 {
		if _, exists := map2[key]; !exists {
			return false
		}
	}

	for key := range map2 {
		if _, exists := map1[key]; !exists {
			return false
		}
	}

	return true
}

func ProcessOutgoingHttp(request []byte, clientConn, destConn net.Conn, h *hooks.Hook, logger *zap.Logger, ctx context.Context) {
	switch models.GetMode() {
	case models.MODE_RECORD:
		err := encodeOutgoingHttp(request, clientConn, destConn, logger, h, ctx)
		if err != nil {
			logger.Error("failed to encode the http message into the yaml", zap.Error(err))
			return
		}

	case models.MODE_TEST:
		decodeOutgoingHttp(request, clientConn, destConn, h, logger)
	default:
		logger.Info("Invalid mode detected while intercepting outgoing http call", zap.Any("mode", models.GetMode()))
	}

}

// Handled chunked requests when content-length is given.
func contentLengthRequest(finalReq *[]byte, clientConn, destConn net.Conn, logger *zap.Logger, contentLength int) error {
	for contentLength > 0 {
		clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		requestChunked, err := util.ReadBytes(clientConn)
		if err != nil {
			if err == io.EOF {
				logger.Error("connection closed by the user client")
				return err
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				logger.Info("Stopped getting data from the connection", zap.Error(err))
				break
			} else {
				logger.Error("failed to read the response message from the destination server")
				return err
			}
		}
		logger.Debug("This is a chunk of request[content-length]: " + string(requestChunked))
		*finalReq = append(*finalReq, requestChunked...)
		contentLength -= len(requestChunked)

		// destConn is nil in case of test mode.
		if destConn != nil {
			_, err = destConn.Write(requestChunked)
			if err != nil {
				logger.Error("failed to write request message to the destination server")
				return err
			}
		}
	}
	return nil
}

// Handled chunked requests when transfer-encoding is given.
func chunkedRequest(finalReq *[]byte, clientConn, destConn net.Conn, logger *zap.Logger, transferEncodingHeader string) error {
	if transferEncodingHeader == "chunked" {
		for {
			//TODO: we have to implement a way to read the buffer chunk wise according to the chunk size (chunk size comes in hexadecimal)
			// because it can happen that some chunks come after 5 seconds.
			clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			requestChunked, err := util.ReadBytes(clientConn)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					break
				} else {
					logger.Error("failed to read the response message from the destination server")
					return err
				}
			}

			*finalReq = append(*finalReq, requestChunked...)
			// destConn is nil in case of test mode.
			if destConn != nil {
				_, err = destConn.Write(requestChunked)
				if err != nil {
					logger.Error("failed to write request message to the destination server")
					return err
				}
			}

			//check if the intial request is completed
			if strings.HasSuffix(string(requestChunked), "0\r\n\r\n") {
				return nil
			}
		}
	}
	return nil
}

// Handled chunked responses when content-length is given.
func contentLengthResponse(finalResp *[]byte, clientConn, destConn net.Conn, logger *zap.Logger, contentLength int) error {
	isEOF := false
	for contentLength > 0 {
		//Set deadline of 5 seconds
		destConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		resp, err := util.ReadBytes(destConn)
		if err != nil {
			if err == io.EOF {
				isEOF = true
				logger.Debug("recieved EOF, connection closed by the destination server")
				if len(resp) == 0 {
					break
				}
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				logger.Info("Stopped getting data from the connection", zap.Error(err))
				break
			} else {
				logger.Error("failed to read the response message from the destination server")
				return err
			}
		}

		logger.Debug("This is a chunk of response[content-length]: " + string(resp))
		*finalResp = append(*finalResp, resp...)
		contentLength -= len(resp)

		// write the response message to the user client
		_, err = clientConn.Write(resp)
		if err != nil {
			logger.Error("failed to write response message to the user client")
			return err
		}

		if isEOF {
			break
		}
	}
	return nil
}

// Handled chunked responses when transfer-encoding is given.
func chunkedResponse(finalResp *[]byte, clientConn, destConn net.Conn, logger *zap.Logger, transferEncodingHeader string) error {
	if transferEncodingHeader == "chunked" {
		isEOF := false
		for {
			resp, err := util.ReadBytes(destConn)
			if err != nil {
				if err != io.EOF {
					logger.Error("failed to read the response message from the destination server", zap.Error(err))
					return err
				} else {
					isEOF = true
					logger.Debug("recieved EOF", zap.Error(err))
					if len(resp) == 0 {
						logger.Debug("exiting loop as response is complete")
						break
					}
				}
			}

			*finalResp = append(*finalResp, resp...)
			// write the response message to the user client
			_, err = clientConn.Write(resp)
			if err != nil {
				logger.Error("failed to write response message to the user client")
				return err
			}

			//In some cases need to write the response to the client
			// where there is some response before getting the true EOF
			if isEOF {
				break
			}

			if string(resp) == "0\r\n\r\n" {
				break
			}
		}
	}
	return nil
}

func handleChunkedRequests(finalReq *[]byte, clientConn, destConn net.Conn, logger *zap.Logger) error {

	if hasCompleteHeaders(*finalReq) {
		logger.Debug("this request has complete headers in the first chunk itself.")
	}

	for !hasCompleteHeaders(*finalReq) {
		logger.Debug("couldn't get complete headers in first chunk so reading more chunks")
		reqHeader, err := util.ReadBytes(clientConn)
		if err != nil {
			logger.Error("failed to read the request message from the client")
			return err
		} else {
			// destConn is nil in case of test mode
			if destConn != nil {
				_, err = destConn.Write(reqHeader)
				if err != nil {
					logger.Error("failed to write request message to the destination server")
					return err
				}
			}
		}

		*finalReq = append(*finalReq, reqHeader...)
	}

	lines := strings.Split(string(*finalReq), "\n")
	var contentLengthHeader string
	var transferEncodingHeader string
	for _, line := range lines {
		if strings.HasPrefix(line, "Content-Length:") {
			contentLengthHeader = strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			break
		} else if strings.HasPrefix(line, "Transfer-Encoding:") {
			transferEncodingHeader = strings.TrimSpace(strings.TrimPrefix(line, "Transfer-Encoding:"))
			break
		}
	}

	//Handle chunked requests
	if contentLengthHeader != "" {
		contentLength, err := strconv.Atoi(contentLengthHeader)
		if err != nil {
			logger.Error("failed to get the content-length header", zap.Error(err))
			return fmt.Errorf("failed to handle chunked request")
		}
		//Get the length of the body in the request.
		bodyLength := len(*finalReq) - strings.Index(string(*finalReq), "\r\n\r\n") - 4
		contentLength -= bodyLength
		if contentLength > 0 {
			err := contentLengthRequest(finalReq, clientConn, destConn, logger, contentLength)
			if err != nil {
				return err
			}
		}
	} else if transferEncodingHeader != "" {
		// check if the intial request is the complete request.
		if strings.HasSuffix(string(*finalReq), "0\r\n\r\n") {
			return nil
		}
		err := chunkedRequest(finalReq, clientConn, destConn, logger, transferEncodingHeader)
		if err != nil {
			return err
		}
	}
	return nil
}

func handleChunkedResponses(finalResp *[]byte, clientConn, destConn net.Conn, logger *zap.Logger, resp []byte) error {

	if hasCompleteHeaders(*finalResp) {
		logger.Debug("this response has complete headers in the first chunk itself.")
	}

	for !hasCompleteHeaders(resp) {
		logger.Debug("couldn't get complete headers in first chunk so reading more chunks")
		respHeader, err := util.ReadBytes(destConn)
		if err != nil {
			if err == io.EOF {
				logger.Debug("received EOF from the server")
				// if there is any buffer left before EOF, we must send it to the client and save this as mock
				if len(respHeader) != 0 {

					// write the response message to the user client
					_, err = clientConn.Write(resp)
					if err != nil {
						logger.Error("failed to write response message to the user client")
						return err
					}
					*finalResp = append(*finalResp, respHeader...)
				}
				return err
			} else {
				logger.Error("failed to read the response message from the destination server")
				return err
			}
		} else {
			// write the response message to the user client
			_, err = clientConn.Write(respHeader)
			if err != nil {
				logger.Error("failed to write response message to the user client")
				return err
			}
		}

		*finalResp = append(*finalResp, respHeader...)
		resp = append(resp, respHeader...)
	}

	//Getting the content-length or the transfer-encoding header
	var contentLengthHeader, transferEncodingHeader string
	lines := strings.Split(string(resp), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Content-Length:") {
			contentLengthHeader = strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			break
		} else if strings.HasPrefix(line, "Transfer-Encoding:") {
			transferEncodingHeader = strings.TrimSpace(strings.TrimPrefix(line, "Transfer-Encoding:"))
			break
		}
	}
	//Handle chunked responses
	if contentLengthHeader != "" {
		contentLength, err := strconv.Atoi(contentLengthHeader)
		if err != nil {
			logger.Error("failed to get the content-length header", zap.Error(err))
			return fmt.Errorf("failed to handle chunked response")
		}
		bodyLength := len(resp) - strings.Index(string(resp), "\r\n\r\n") - 4
		contentLength -= bodyLength
		if contentLength > 0 {
			err := contentLengthResponse(finalResp, clientConn, destConn, logger, contentLength)
			if err != nil {
				return err
			}
		}
	} else if transferEncodingHeader != "" {
		//check if the intial response is the complete response.
		if strings.HasSuffix(string(*finalResp), "0\r\n\r\n") {
			return nil
		}
		err := chunkedResponse(finalResp, clientConn, destConn, logger, transferEncodingHeader)
		if err != nil {
			return err
		}
	}
	return nil
}

// Checks if the response is gzipped
func checkIfGzipped(check io.ReadCloser) (bool, *bufio.Reader) {
	bufReader := bufio.NewReader(check)
	peekedBytes, err := bufReader.Peek(2)
	if err != nil && err != io.EOF {
		log.Debug("Error peeking:", err)
		return false, nil
	}
	if len(peekedBytes) < 2 {
		return false, nil
	}
	if peekedBytes[0] == 0x1f && peekedBytes[1] == 0x8b {
		return true, bufReader
	} else {
		return false, nil
	}
}

// Decodes the mocks in test mode so that they can be sent to the user application.
func decodeOutgoingHttp(requestBuffer []byte, clientConn, destConn net.Conn, h *hooks.Hook, logger *zap.Logger) {
	//Matching algorithmm
	//Get the mocks
	for {

		//Check if the expected header is present
		if bytes.Contains(requestBuffer, []byte("Expect: 100-continue")) {
			//Send the 100 continue response
			_, err := clientConn.Write([]byte("HTTP/1.1 100 Continue\r\n\r\n"))
			if err != nil {
				logger.Error("failed to write the 100 continue response to the user application", zap.Error(err))
				return
			}
			//Read the request buffer again
			newRequest, err := util.ReadBytes(clientConn)
			if err != nil {
				logger.Error("failed to read the request buffer from the user application", zap.Error(err))
				return
			}
			//Append the new request buffer to the old request buffer
			requestBuffer = append(requestBuffer, newRequest...)
		}

		err := handleChunkedRequests(&requestBuffer, clientConn, destConn, logger)
		if err != nil {
			logger.Error("failed to handle chunk request", zap.Error(err))
			return
		}

		logger.Debug(fmt.Sprintf("This is the complete request:\n%v", string(requestBuffer)))

		//Parse the request buffer
		req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(requestBuffer)))
		if err != nil {
			logger.Error("failed to parse the http request message", zap.Error(err))
			return
		}

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed to read from request body", zap.Error(err))
			return
		}

		//parse request url
		reqURL, err := url.Parse(req.URL.String())
		if err != nil {
			logger.Error("failed to parse request url", zap.Error(err))
			return
		}

		//check if req body is a json
		isReqBodyJSON := isJSON(reqBody)

		isMatched, stub, err := match(req, reqBody, reqURL, isReqBodyJSON, h, logger, clientConn, destConn, requestBuffer, h.Recover)

		if err != nil {
			logger.Error("error while matching http mocks", zap.Error(err))
		}

		if !isMatched {
			passthroughHost := false
			for _, host := range models.PassThroughHosts {
				if req.Host == host {
					passthroughHost = true
				}
			}
			if !passthroughHost {
				logger.Error("Didn't match any prexisting http mock")
			}
			_, err := util.Passthrough(clientConn, destConn, [][]byte{requestBuffer}, h.Recover, logger)
			if err != nil {
				logger.Error("failed to passthrough http request", zap.Error(err))
			}
			return
		}

		statusLine := fmt.Sprintf("HTTP/%d.%d %d %s\r\n", stub.Spec.HttpReq.ProtoMajor, stub.Spec.HttpReq.ProtoMinor, stub.Spec.HttpResp.StatusCode, http.StatusText(int(stub.Spec.HttpResp.StatusCode)))

		body := stub.Spec.HttpResp.Body
		var respBody string
		var responseString string

		// Fetching the response headers
		header := pkg.ToHttpHeader(stub.Spec.HttpResp.Header)

		//Check if the gzip encoding is present in the header
		if header["Content-Encoding"] != nil && header["Content-Encoding"][0] == "gzip" {
			var compressedBuffer bytes.Buffer
			gw := gzip.NewWriter(&compressedBuffer)
			_, err := gw.Write([]byte(body))
			if err != nil {
				logger.Error("failed to compress the response body", zap.Error(err))
				return
			}
			err = gw.Close()
			if err != nil {
				logger.Error("failed to close the gzip writer", zap.Error(err))
				return
			}
			logger.Debug("the length of the response body: " + strconv.Itoa(len(compressedBuffer.String())))
			respBody = compressedBuffer.String()
			// responseString = statusLine + headers + "\r\n" + compressedBuffer.String()
		} else {
			respBody = body
			// responseString = statusLine + headers + "\r\n" + body
		}
		var headers string
		for key, values := range header {
			if key == "Content-Length" {
				values = []string{strconv.Itoa(len(respBody))}
			}
			for _, value := range values {
				headerLine := fmt.Sprintf("%s: %s\r\n", key, value)
				headers += headerLine
			}
		}
		responseString = statusLine + headers + "\r\n" + "" + respBody

		logger.Debug(fmt.Sprintf("Mock Response sending back to client:\n%v", responseString))

		_, err = clientConn.Write([]byte(responseString))
		if err != nil {
			logger.Error("failed to write the mock output to the user application", zap.Error(err))
			return
		}

		requestBuffer, err = util.ReadBytes(clientConn)
		if err != nil {
			logger.Debug("failed to read the request buffer from the client", zap.Error(err))
			logger.Debug("This was the last response from the server:\n" + string(responseString))
			break
		}

	}

}

// encodeOutgoingHttp function parses the HTTP request and response text messages to capture outgoing network calls as mocks.
func encodeOutgoingHttp(request []byte, clientConn, destConn net.Conn, logger *zap.Logger, h *hooks.Hook, ctx context.Context) error {
	var resp []byte
	var finalResp []byte
	var finalReq []byte
	var err error

	//closing the destination connection
	defer destConn.Close()

	//Writing the request to the server.
	_, err = destConn.Write(request)
	if err != nil {
		logger.Error("failed to write request message to the destination server", zap.Error(err))
		return err
	}

	logger.Debug("This is the initial request: " + string(request))
	finalReq = append(finalReq, request...)

	//for keeping the connection alive
	for {
		//check if the expect : 100-continue header is present
		lines := strings.Split(string(finalReq), "\n")
		var expectHeader string
		for _, line := range lines {
			if strings.HasPrefix(line, "Expect:") {
				expectHeader = strings.TrimSpace(strings.TrimPrefix(line, "Expect:"))
				break
			}
		}
		if expectHeader == "100-continue" {
			//Read if the response from the server is 100-continue
			resp, err = util.ReadBytes(destConn)
			if err != nil {
				logger.Error("failed to read the response message from the server after 100-continue request", zap.Error(err))
				return err
			}

			// write the response message to the client
			_, err = clientConn.Write(resp)
			if err != nil {
				logger.Error("failed to write response message to the user client", zap.Error(err))
				return err
			}

			logger.Debug("This is the response from the server after the expect header" + string(resp))

			if string(resp) != "HTTP/1.1 100 Continue\r\n\r\n" {
				logger.Error("failed to get the 100 continue response from the user client")
				return err
			}
			//Reading the request buffer again
			request, err = util.ReadBytes(clientConn)
			if err != nil {
				logger.Error("failed to read the request message from the user client", zap.Error(err))
				return err
			}
			// write the request message to the actual destination server
			_, err = destConn.Write(request)
			if err != nil {
				logger.Error("failed to write request message to the destination server", zap.Error(err))
				return err
			}
			finalReq = append(finalReq, request...)
		}

		// Capture the request timestamp
		reqTimestampMock := time.Now()

		err := handleChunkedRequests(&finalReq, clientConn, destConn, logger)
		if err != nil {
			logger.Error("failed to handle chunk request", zap.Error(err))
			return err
		}

		logger.Debug(fmt.Sprintf("This is the complete request:\n%v", string(finalReq)))
		// read the response from the actual server
		resp, err = util.ReadBytes(destConn)
		if err != nil {
			if err == io.EOF {
				logger.Debug("Response complete, exiting the loop.")
				// if there is any buffer left before EOF, we must send it to the client and save this as mock
				if len(resp) != 0 {

					// Capturing the response timestamp
					resTimestampcMock := time.Now()
					// write the response message to the user client
					_, err = clientConn.Write(resp)
					if err != nil {
						logger.Error("failed to write response message to the user client", zap.Error(err))
						return err
					}

					// saving last request/response on this connection.
					err := ParseFinalHttp(finalReq, finalResp, reqTimestampMock, resTimestampcMock, ctx, logger, h)
					if err != nil {
						logger.Error("failed to parse the final http request and response", zap.Error(err))
						return err
					}
				}
				break
			} else {
				logger.Error("failed to read the response message from the destination server", zap.Error(err))
				return err
			}
		}

		// Capturing the response timestamp
		resTimestampcMock := time.Now()

		// write the response message to the user client
		_, err = clientConn.Write(resp)
		if err != nil {
			logger.Error("failed to write response message to the user client", zap.Error(err))
			return err
		}

		finalResp = append(finalResp, resp...)
		logger.Debug("This is the initial response: " + string(resp))

		err = handleChunkedResponses(&finalResp, clientConn, destConn, logger, resp)
		if err != nil {
			if err == io.EOF {
				logger.Debug("connection closed by the server", zap.Error(err))
				//check if before EOF complete response came, and try to parse it.
				parseErr := ParseFinalHttp(finalReq, finalResp, reqTimestampMock, resTimestampcMock, ctx, logger, h)
				if parseErr != nil {
					logger.Error("failed to parse the final http request and response", zap.Error(parseErr))
					return parseErr
				}
				return nil
			} else {
				logger.Error("failed to handle chunk response", zap.Error(err))
				return err
			}
		}

		logger.Debug("This is the final response: " + string(finalResp))

		err = ParseFinalHttp(finalReq, finalResp, reqTimestampMock, resTimestampcMock, ctx, logger, h)
		if err != nil {
			logger.Error("failed to parse the final http request and response", zap.Error(err))
			return err
		}

		//resetting for the new request and response.
		finalReq = []byte("")
		finalResp = []byte("")

		finalReq, err = util.ReadBytes(clientConn)
		if err != nil {
			if err != io.EOF {
				logger.Debug("failed to read the request message from the user client", zap.Error(err))
				logger.Debug("This was the last response from the server: " + string(resp))
			}
			break
		}
		// write the request message to the actual destination server
		_, err = destConn.Write(finalReq)
		if err != nil {
			logger.Error("failed to write request message to the destination server", zap.Error(err))
			return err
		}
	}
	return nil
}

// ParseFinalHttp is used to parse the final http request and response and save it in a yaml file
func ParseFinalHttp(finalReq []byte, finalResp []byte, reqTimestampMock, resTimestampcMock time.Time, ctx context.Context, logger *zap.Logger, h *hooks.Hook) error {
	var req *http.Request
	// converts the request message buffer to http request
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(finalReq)))
	if err != nil {
		logger.Error("failed to parse the http request message", zap.Error(err))
		return err
	}
	var reqBody []byte
	if req.Body != nil { // Read
		var err error
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			// TODO right way to log errors
			logger.Error("failed to read the http request body", zap.Error(err))
			return err
		}
	}
	// converts the response message buffer to http response
	respParsed, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(finalResp)), req)
	if err != nil {
		logger.Error("failed to parse the http response message", zap.Error(err))
		return err
	}
	//Add the content length to the headers.
	var respBody []byte
	//Checking if the body of the response is empty or does not exist.

	if respParsed.Body != nil { // Read
		if respParsed.Header.Get("Content-Encoding") == "gzip" {
			check := respParsed.Body
			ok, reader := checkIfGzipped(check)
			logger.Debug("The body is gzip? " + strconv.FormatBool(ok))
			logger.Debug("", zap.Any("isGzipped", ok))
			if ok {
				gzipReader, err := gzip.NewReader(reader)
				if err != nil {
					logger.Error("failed to create a gzip reader", zap.Error(err))
					return err
				}
				respParsed.Body = gzipReader
			}
		}
		respBody, err = io.ReadAll(respParsed.Body)
		if err != nil {
			logger.Error("failed to read the the http response body", zap.Error(err))
			return err
		}
		logger.Debug("This is the response body: " + string(respBody))
		//Set the content length to the headers.
		respParsed.Header.Set("Content-Length", strconv.Itoa(len(respBody)))
	}
	// store the request and responses as mocks
	meta := map[string]string{
		"name":      "Http",
		"type":      models.HttpClient,
		"operation": req.Method,
	}
	passthroughHost := false
	for _, host := range models.PassThroughHosts {
		if req.Host == host {
			passthroughHost = true
		}
	}
	if !passthroughHost {
		go func() {
			// Recover from panic and gracefully shutdown
			defer h.Recover(pkg.GenerateRandomID())
			defer utils.HandlePanic()
			err := h.AppendMocks(&models.Mock{
				Version: models.GetVersion(),
				Name:    "mocks",
				Kind:    models.HTTP,
				Spec: models.MockSpec{
					Metadata: meta,
					HttpReq: &models.HttpReq{
						Method:     models.Method(req.Method),
						ProtoMajor: req.ProtoMajor,
						ProtoMinor: req.ProtoMinor,
						URL:        req.URL.String(),
						Header:     pkg.ToYamlHttpHeader(req.Header),
						Body:       string(reqBody),
						URLParams:  pkg.UrlParams(req),
						Host:       req.Host,
					},
					HttpResp: &models.HttpResp{
						StatusCode: respParsed.StatusCode,
						Header:     pkg.ToYamlHttpHeader(respParsed.Header),
						Body:       string(respBody),
					},
					Created:          time.Now().Unix(),
					ReqTimestampMock: reqTimestampMock,
					ResTimestampMock: resTimestampcMock,
				},
			}, ctx)

			if err != nil {
				logger.Error("failed to store the http mock", zap.Error(err))
			}
		}()

	}
	return nil
}

// hasCompleteHeaders checks if the given byte slice contains the complete HTTP headers
func hasCompleteHeaders(httpChunk []byte) bool {
	// Define the sequence for header end: "\r\n\r\n"
	headerEndSequence := []byte{'\r', '\n', '\r', '\n'}

	// Check if the byte slice contains the header end sequence
	return bytes.Contains(httpChunk, headerEndSequence)
}
