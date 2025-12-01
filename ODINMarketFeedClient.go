// Package ODINMarketFeed provides a Go Library for connecting to ODIN Market Feed WebSocket API.
// This library enables real-time market data streaming with built-in compression,
// fragmentation handling, and subscription management for trading applications.
//
// Features:
//   - WebSocket connectivity with SSL/TLS support
//   - ZLIB compression and decompression
//   - Automatic message fragmentation handling
//   - Multiple subscription types (ChannelNum, SnapQuote, BestFive)
//   - Heartbeat mechanism for connection maintenance
//   - Thread-safe operations
//   - Binary data parsing for market data
//
// Basic Usage:
//
//	client := odinclient.NewODINMarketFeedClient()
//
//	client.OnOpen = func() {
//	    fmt.Println("Connected")
//	    client.SubscribeChannelNum("1234,5678", 1)
//	}
//
//	client.OnMessage = func(message string) {
//	    fmt.Println("Received:", message)
//	}
//
//	client.Connect("host.com", 8080, true, "user_id")
//	defer client.Disconnect()
//
// For detailed examples and documentation, visit:
// https://github.com/SIPL-Dev/go-odinmarketfeedclient
package ODINMarketFeed

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// CompressionStatus represents compression state
type CompressionStatus int

const (
	CompressionON CompressionStatus = iota
	CompressionOFF
)

// MarketData represents market data structure
type MarketData struct {
	MktSegID       uint32
	Token          uint32
	LUT            uint32
	LTP            uint32
	ClosePrice     uint32
	DecimalLocator uint32
}

// ZLIBCompressor handles ZLIB compression/decompression
type ZLIBCompressor struct{}

// Compress compresses data using ZLIB
func (z *ZLIBCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Uncompress decompresses data using ZLIB
func (z *ZLIBCompressor) Uncompress(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, reader)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// FragmentationHandler handles message fragmentation
type FragmentationHandler struct {
	memoryStream        *bytes.Buffer
	lastWrittenIndex    int
	isDisposed          bool
	zlibCompressor      *ZLIBCompressor
	UnCompressMsgLength int
	HeaderLength        int
	mu                  sync.Mutex
	headerChar          []byte
	IsUncompress        bool
}

const (
	minimumPacketSize = 5
	packetHeaderSize  = 5
	//messageLengthLen  = 5
)

// NewFragmentationHandler creates a new FragmentationHandler
func NewFragmentationHandler() *FragmentationHandler {
	return &FragmentationHandler{
		memoryStream:     bytes.NewBuffer(nil),
		lastWrittenIndex: -1,
		isDisposed:       false,
		zlibCompressor:   &ZLIBCompressor{},
		headerChar:       make([]byte, 5),
		IsUncompress:     false,
		HeaderLength:     6,
	}
}

// FragmentData fragments and compresses data for sending
func (fh *FragmentationHandler) FragmentData(data []byte) ([]byte, error) {
	compressed, err := fh.zlibCompressor.Compress(data)
	if err != nil {
		return nil, err
	}

	lengthString := fmt.Sprintf("%06d", len(compressed))
	lenBytes := []byte(lengthString)
	lenBytes[0] = 5 // compression flag

	result := append(lenBytes, compressed...)
	return result, nil
}

// Defragment defragments received data
func (fh *FragmentationHandler) Defragment(data []byte) ([][]byte, error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if fh.isDisposed {
		return nil, nil
	}

	// Write data to memory stream
	fh.memoryStream.Write(data)
	fh.lastWrittenIndex = fh.memoryStream.Len() - 1

	return fh.defragmentData()
}

func (fh *FragmentationHandler) defragmentData() ([][]byte, error) {
	parseDone := false
	bytesParsed := 0
	packetList := make([][]byte, 0)

	streamData := fh.memoryStream.Bytes()
	position := 0
	packetCount := 0

	for position < fh.lastWrittenIndex-minimumPacketSize && !parseDone {
		headerEnd := position + packetHeaderSize + 1
		if headerEnd > len(streamData) {
			break
		}

		header := streamData[position:headerEnd]
		packetSize := fh.isLength(header)

		if packetSize <= 0 {
			position++
			bytesParsed++
		} else {
			dataStart := headerEnd
			dataEnd := dataStart + packetSize

			if dataEnd <= fh.lastWrittenIndex+1 {
				compressData := streamData[dataStart:dataEnd]
				messageData, err := fh.defragmentInnerData(compressData)
				if err == nil {
					//packetList = append(packetList, messageData)
					for {
						fh.UnCompressMsgLength = 0
						fh.UnCompressMsgLength = fh.GetMessageLength(messageData)

						if fh.UnCompressMsgLength <= 0 {
							messageData = nil
							break
						}

						unCompressBytes := make([]byte, fh.UnCompressMsgLength)
						copy(unCompressBytes, messageData[fh.HeaderLength:fh.HeaderLength+fh.UnCompressMsgLength])
						packetList = append(packetList, unCompressBytes)
						packetCount++

						remainingLength := len(messageData) - fh.UnCompressMsgLength - fh.HeaderLength
						if remainingLength <= 0 {
							messageData = nil
							break
						}

						unCompressNewBytes := make([]byte, remainingLength)
						copy(unCompressNewBytes, messageData[fh.UnCompressMsgLength+fh.HeaderLength:])
						messageData = unCompressNewBytes
					}
				}
				bytesParsed += packetHeaderSize + 1 + packetSize
				position = dataEnd
			} else {
				parseDone = true
			}
		}
	}

	fh.clearProcessedData(bytesParsed)
	return packetList, nil
}

func (fh *FragmentationHandler) isLength(header []byte) int {
	if len(header) != packetHeaderSize+1 {
		return -1
	}

	if header[0] != 5 && header[0] != 2 {
		return -1
	}

	lengthStr := string(header[1:6])
	for _, ch := range lengthStr {
		if ch < '0' || ch > '9' {
			return -1
		}
	}

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return -1
	}

	return length
}

// GetMessageLength extracts the message length from the message data
// You'll need to implement this based on your protocol
func (fh *FragmentationHandler) GetMessageLength(messageData []byte) int {
	if len(messageData) == 0 {
		return 0
	}

	// Check first byte to determine compression
	if messageData[0] == 5 {
		fh.IsUncompress = false
	} else {
		fh.IsUncompress = true
	}

	// Extract length from header
	defer func() {
		if r := recover(); r != nil {
			// Handle panic (equivalent to catch block)
		}
	}()

	startIndex := 0
	for i := 1; i < fh.HeaderLength && i < len(messageData); i++ {
		fh.headerChar[startIndex] = messageData[i]
		startIndex++
	}

	// Convert bytes to string and parse as integer
	sLength := string(fh.headerChar[:startIndex])
	iLength, err := strconv.Atoi(sLength)
	if err != nil {
		return 0
	}

	return iLength
}

func (fh *FragmentationHandler) defragmentInnerData(compressData []byte) ([]byte, error) {
	return fh.zlibCompressor.Uncompress(compressData)
}

func (fh *FragmentationHandler) clearProcessedData(length int) {
	if length <= 0 {
		return
	}

	if length >= fh.lastWrittenIndex+1 {
		fh.lastWrittenIndex = -1
		fh.memoryStream = bytes.NewBuffer(nil)
		return
	}

	size := (fh.lastWrittenIndex + 1) - length
	data := fh.memoryStream.Bytes()[length : length+size]
	fh.memoryStream = bytes.NewBuffer(data)
	fh.lastWrittenIndex = size - 1
}

// ODINMarketFeedClient represents the WebSocket client
type ODINMarketFeedClient struct {
	conn              *websocket.Conn
	compressionStatus CompressionStatus
	channelID         string
	userID            string
	isDisposed        bool
	receiveBufferSize int
	dteNSE            time.Time
	fragHandler       *FragmentationHandler

	OnOpen    func()
	OnMessage func(message string)
	OnError   func(err string)
	OnClose   func(code int, reason string)

	mu sync.Mutex
}

// NewODINMarketFeedClient creates a new ODINMarketFeedClient instance
func NewODINMarketFeedClient() *ODINMarketFeedClient {
	return &ODINMarketFeedClient{
		compressionStatus: CompressionON,
		channelID:         "Broadcast",
		receiveBufferSize: 8192,
		fragHandler:       NewFragmentationHandler(),
		dteNSE:            time.Date(1980, 1, 1, 0, 0, 0, 0, time.Local),
	}
}

// SetCompression enables or disables compression
func (tw *ODINMarketFeedClient) SetCompression(enabled bool) {
	if enabled {
		tw.compressionStatus = CompressionON
	} else {
		tw.compressionStatus = CompressionOFF
	}
}

// Connect connects to the WebSocket server
func (tw *ODINMarketFeedClient) Connect(host string, port int, useSSL bool, userID string, apiKey string) error {

	// Validate host
	if strings.TrimSpace(host) == "" {
		return errors.New("host cannot be empty")
	}

	// Validate host format (hostname or IP address)
	if net.ParseIP(host) == nil {
		// Not a valid IP, check if it's a valid hostname
		hostnameRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)
		if !hostnameRegex.MatchString(host) {
			return fmt.Errorf("invalid host format: %s", host)
		}
	}

	// Validate port range
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got: %d", port)
	}

	// Validate userID
	if strings.TrimSpace(userID) == "" {
		return errors.New("userID cannot be empty")
	}

	// Validate userID length

	if len(userID) > 12 {
		return errors.New("userID is too long (max 12 characters)")
	}

	tw.userID = userID
	protocol := "ws"
	if useSSL {
		protocol = "wss"
	}
	url := fmt.Sprintf("%s://%s:%d", protocol, host, port)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		errMsg := fmt.Sprintf("Connection failed: %v", err)
		if tw.OnError != nil {
			tw.OnError(errMsg)
		}
		return err
	}

	tw.conn = conn
	fmt.Println("Connected")

	// Start receiving messages
	go tw.receiveMessages()

	currentTime := tw.formatTime(time.Now())

	password := "68="
	if apiKey != "" && strings.TrimSpace(apiKey) != "" {
		password = fmt.Sprintf("68=%s|401=2", apiKey)
	}

	// Build login message
	loginMsg := fmt.Sprintf("63=FT3.0|64=101|65=74|66=%s|67=%s|%s", currentTime, userID, password)
	// Send login message
	//loginMsg := fmt.Sprintf("63=FT3.0|64=101|65=74|66=14:59:22|67=%s|68=|4=|400=0|396=HO|51=4|395=127.0.0.1", tw.userID)
	err = tw.SendMessage(loginMsg)
	if err != nil {
		return err
	}

	if tw.OnOpen != nil {
		tw.OnOpen()
	}

	return nil
}

// Disconnect disconnects from the WebSocket server
func (tw *ODINMarketFeedClient) Disconnect() error {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.conn != nil {
		err := tw.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			return err
		}
		return tw.conn.Close()
	}
	return nil
}

// SubscribeTouchline subscribes to touchline for the provided tokens
func (tw *ODINMarketFeedClient) SubscribeTouchlineOld(tokenList []string) error {
	if tokenList == nil || len(tokenList) == 0 {
		errMsg := "Token list cannot be null or empty."
		if tw.OnError != nil {
			tw.OnError(errMsg)
		}
		return fmt.Errorf(errMsg)
	}

	strTokenToSubscribe := ""

	for _, item := range tokenList {
		if strings.TrimSpace(item) == "" {
			continue
		}

		parts := strings.Split(item, "_")
		if len(parts) != 2 {
			errMsg := fmt.Sprintf("Invalid token format: '%s'. Expected format: 'MarketSegmentID_Token'.", item)
			if tw.OnError != nil {
				tw.OnError(errMsg)
			}
			continue
		}

		marketSegmentID, err1 := strconv.Atoi(parts[0])
		token, err2 := strconv.Atoi(parts[1])

		if err1 != nil || err2 != nil {
			errMsg := fmt.Sprintf("Invalid token format: '%s'. Expected format: 'MarketSegmentID_Token'.", item)
			if tw.OnError != nil {
				tw.OnError(errMsg)
			}
			continue
		}

		strTokenToSubscribe += fmt.Sprintf("1=%d$7=%d|", marketSegmentID, token)
	}

	if strTokenToSubscribe != "" {
		currentTime := time.Now().Format("15:04:05")
		tlRequest := fmt.Sprintf("63=FT3.0|64=206|65=84|66=%s|4=|%s230=1", currentTime, strTokenToSubscribe)

		err := tw.SendMessage(tlRequest)
		if err != nil {
			return err
		}

		fmt.Printf("Subscribed to touchline tokens: %s\n", strings.Join(tokenList, ", "))
		return nil
	}

	errMsg := "No valid tokens found to subscribe."
	if tw.OnError != nil {
		tw.OnError(errMsg)
	}
	return fmt.Errorf(errMsg)
}

// SubscribeTouchline sends touchline request for market data
// tokenList: List of tokens to subscribe (e.g., "1_22", "1_2885")
// responseType: "1" = Touchline with fixed length native data, "0" = Normal touchline
// ltpChangeOnly: Send response on LTP change only if true
func (tw *ODINMarketFeedClient) SubscribeTouchline(tokenList []string, responseType string, ltpChangeOnly bool) error {
	if len(tokenList) == 0 {
		if tw.OnError != nil {
			tw.OnError("Token list cannot be null or empty.")
		}
		return fmt.Errorf("token list cannot be empty")
	}

	if responseType != "0" && responseType != "1" {
		if tw.OnError != nil {
			tw.OnError("Invalid response type passed. Valid values are 0 or 1")
		}
		return fmt.Errorf("invalid response type")
	}

	var strTokenToSubscribe strings.Builder

	for _, item := range tokenList {
		if tw.isNullOrWhiteSpace(item) {
			continue
		}

		parts := strings.Split(item, "_")

		if len(parts) != 2 {
			if tw.OnError != nil {
				tw.OnError(fmt.Sprintf("Invalid token format: '%s'. Expected format: 'MarketSegmentID_Token'.", item))
			}
			continue
		}

		marketSegmentID, err1 := strconv.Atoi(parts[0])
		token, err2 := strconv.Atoi(parts[1])

		if err1 != nil || err2 != nil {
			if tw.OnError != nil {
				tw.OnError(fmt.Sprintf("Invalid token format: '%s'. Expected format: 'MarketSegmentID_Token'.", item))
			}
			continue
		}

		strTokenToSubscribe.WriteString(fmt.Sprintf("1=%d$7=%d|", marketSegmentID, token))
	}

	strResponseType := ""
	if responseType == "1" {
		strResponseType = "49=1"
	}

	sLTChangeOnly := "200=0"
	if ltpChangeOnly {
		sLTChangeOnly = "200=1"
	}

	if strTokenToSubscribe.Len() > 0 {
		currentTime := tw.formatTime(time.Now())
		var tlRequest string

		if strResponseType != "" {
			tlRequest = fmt.Sprintf("63=FT3.0|64=206|65=84|66=%s|%s|%s|%s230=1",
				currentTime, strResponseType, sLTChangeOnly, strTokenToSubscribe.String())
		} else {
			tlRequest = fmt.Sprintf("63=FT3.0|64=206|65=84|66=%s|%s|%s230=1",
				currentTime, sLTChangeOnly, strTokenToSubscribe.String())
		}

		if err := tw.SendMessage(tlRequest); err != nil {
			return err
		}
		fmt.Printf("Subscribed to touchline tokens: %s\n", strings.Join(tokenList, ", "))
		return nil
	}

	if tw.OnError != nil {
		tw.OnError("No valid tokens found to subscribe.")
	}
	return fmt.Errorf("no valid tokens found")
}

// SubscribeLTPTouchline sends LTP touchline request for market data
// tokenList: List of tokens to subscribe (e.g., "1_22", "1_2885")
func (c *ODINMarketFeedClient) SubscribeLTPTouchline(tokenList []string) error {
	if len(tokenList) == 0 {
		if c.OnError != nil {
			c.OnError("Token list cannot be null or empty.")
		}
		return fmt.Errorf("token list cannot be empty")
	}

	var strTokenToSubscribe strings.Builder

	for _, item := range tokenList {
		if c.isNullOrWhiteSpace(item) {
			continue
		}

		parts := strings.Split(item, "_")

		if len(parts) != 2 {
			if c.OnError != nil {
				c.OnError(fmt.Sprintf("Invalid token format: '%s'. Expected format: 'MarketSegmentID_Token'.", item))
			}
			continue
		}

		marketSegmentID, err1 := strconv.Atoi(parts[0])
		token, err2 := strconv.Atoi(parts[1])

		if err1 != nil || err2 != nil {
			if c.OnError != nil {
				c.OnError(fmt.Sprintf("Invalid token format: '%s'. Expected format: 'MarketSegmentID_Token'.", item))
			}
			continue
		}

		strTokenToSubscribe.WriteString(fmt.Sprintf("1=%d$7=%d|", marketSegmentID, token))
	}

	if strTokenToSubscribe.Len() > 0 {
		currentTime := c.formatTime(time.Now())
		tlRequest := fmt.Sprintf("63=FT3.0|64=347|65=84|66=%s|%s230=1",
			currentTime, strTokenToSubscribe.String())

		if err := c.SendMessage(tlRequest); err != nil {
			return err
		}
		fmt.Printf("Subscribed to LTP touchline tokens: %s\n", strings.Join(tokenList, ", "))
		return nil
	}

	if c.OnError != nil {
		c.OnError("No valid tokens found to subscribe.")
	}
	return fmt.Errorf("no valid tokens found")
}

// UnsubscribeLTPTouchline unsubscribes from LTP touchline tokens
func (c *ODINMarketFeedClient) UnsubscribeLTPTouchline(tokenList []string) error {
	if len(tokenList) == 0 {
		if c.OnError != nil {
			c.OnError("Token list cannot be null or empty.")
		}
		return fmt.Errorf("token list cannot be empty")
	}

	var strTokenToSubscribe strings.Builder

	for _, item := range tokenList {
		if c.isNullOrWhiteSpace(item) {
			continue
		}

		parts := strings.Split(item, "_")

		if len(parts) != 2 {
			if c.OnError != nil {
				c.OnError(fmt.Sprintf("Invalid token format: '%s'. Expected format: 'MarketSegmentID_Token'.", item))
			}
			continue
		}

		marketSegmentID, err1 := strconv.Atoi(parts[0])
		token, err2 := strconv.Atoi(parts[1])

		if err1 != nil || err2 != nil {
			if c.OnError != nil {
				c.OnError(fmt.Sprintf("Invalid token format: '%s'. Expected format: 'MarketSegmentID_Token'.", item))
			}
			continue
		}

		strTokenToSubscribe.WriteString(fmt.Sprintf("1=%d$7=%d|", marketSegmentID, token))
	}

	if strTokenToSubscribe.Len() > 0 {
		currentTime := c.formatTime(time.Now())
		tlRequest := fmt.Sprintf("63=FT3.0|64=347|65=84|66=%s|%s230=2",
			currentTime, strTokenToSubscribe.String())

		if err := c.SendMessage(tlRequest); err != nil {
			return err
		}
		fmt.Printf("Unsubscribed from LTP touchline tokens: %s\n", strings.Join(tokenList, ", "))
		return nil
	}

	if c.OnError != nil {
		c.OnError("No valid tokens found to subscribe.")
	}
	return fmt.Errorf("no valid tokens found")
}

// SubscribePauseResume pauses or resumes the broadcast subscription
// isPause: true to pause, false to resume
func (c *ODINMarketFeedClient) SubscribePauseResume(isPause bool) error {
	sIsPause := "230=2"
	if isPause {
		sIsPause = "230=1"
	}

	currentTime := c.formatTime(time.Now())
	tlRequest := fmt.Sprintf("63=FT3.0|64=106|65=84|66=%s|%s", currentTime, sIsPause)

	if err := c.SendMessage(tlRequest); err != nil {
		return err
	}

	action := "Resume"
	if isPause {
		action = "Pause"
	}
	fmt.Printf("%s request sent\n", action)
	return nil
}

// Helper methods

func (c *ODINMarketFeedClient) isNullOrWhiteSpace(str string) bool {
	return len(strings.TrimSpace(str)) == 0
}

func (c *ODINMarketFeedClient) formatTime(t time.Time) string {
	return t.Format("15:04:05")
}

// UnsubscribeTouchline unsubscribes from touchline for the provided tokens
func (tw *ODINMarketFeedClient) UnsubscribeTouchline(tokenList []string) error {
	if tokenList == nil || len(tokenList) == 0 {
		errMsg := "Token list cannot be null or empty."
		if tw.OnError != nil {
			tw.OnError(errMsg)
		}
		return fmt.Errorf(errMsg)
	}

	strTokenToSubscribe := ""

	for _, item := range tokenList {
		if strings.TrimSpace(item) == "" {
			continue
		}

		parts := strings.Split(item, "_")
		if len(parts) != 2 {
			errMsg := fmt.Sprintf("Invalid token format: '%s'. Expected format: 'MarketSegmentID_Token'.", item)
			if tw.OnError != nil {
				tw.OnError(errMsg)
			}
			continue
		}

		marketSegmentID, err1 := strconv.Atoi(parts[0])
		token, err2 := strconv.Atoi(parts[1])

		if err1 != nil || err2 != nil {
			errMsg := fmt.Sprintf("Invalid token format: '%s'. Expected format: 'MarketSegmentID_Token'.", item)
			if tw.OnError != nil {
				tw.OnError(errMsg)
			}
			continue
		}

		strTokenToSubscribe += fmt.Sprintf("1=%d$7=%d|", marketSegmentID, token)
	}

	if strTokenToSubscribe != "" {
		currentTime := time.Now().Format("15:04:05")
		tlRequest := fmt.Sprintf("63=FT3.0|64=206|65=84|66=%s|4=|%s230=2", currentTime, strTokenToSubscribe)

		err := tw.SendMessage(tlRequest)
		if err != nil {
			return err
		}

		fmt.Printf("Unsubscribed from touchline tokens: %s\n", strings.Join(tokenList, ", "))
		return nil
	}

	errMsg := "No valid tokens found to unsubscribe."
	if tw.OnError != nil {
		tw.OnError(errMsg)
	}
	return fmt.Errorf(errMsg)
}

// SubscribeBestFive subscribes to Market Depth (Best Five) for the provided token and market segment
func (tw *ODINMarketFeedClient) SubscribeBestFive(token string, marketSegmentID int) error {
	if strings.TrimSpace(token) == "" {
		errMsg := "Token cannot be null or empty."
		if tw.OnError != nil {
			tw.OnError(errMsg)
		}
		return fmt.Errorf(errMsg)
	}

	if marketSegmentID <= 0 {
		errMsg := "Invalid MarketSegment."
		if tw.OnError != nil {
			tw.OnError(errMsg)
		}
		return fmt.Errorf(errMsg)
	}

	currentTime := time.Now().Format("15:04:05")
	tlRequest := fmt.Sprintf("63=FT3.0|64=127|65=84|66=%s|1=%d|7=%s|230=1", currentTime, marketSegmentID, token)

	err := tw.SendMessage(tlRequest)
	if err != nil {
		return err
	}

	fmt.Printf("Subscribed to BestFive tokens: %s, MarketSegmentId: %d\n", token, marketSegmentID)
	return nil
}

// UnsubscribeBestFive unsubscribes from Market Depth (Best Five) for the provided token and market segment
func (tw *ODINMarketFeedClient) UnsubscribeBestFive(token string, marketSegmentID int) error {
	if strings.TrimSpace(token) == "" {
		errMsg := "Token cannot be null or empty."
		if tw.OnError != nil {
			tw.OnError(errMsg)
		}
		return fmt.Errorf(errMsg)
	}

	if marketSegmentID <= 0 {
		errMsg := "Invalid MarketSegment."
		if tw.OnError != nil {
			tw.OnError(errMsg)
		}
		return fmt.Errorf(errMsg)
	}

	currentTime := time.Now().Format("15:04:05")
	tlRequest := fmt.Sprintf("63=FT3.0|64=127|65=84|66=%s|1=%d|7=%s|230=2", currentTime, marketSegmentID, token)

	err := tw.SendMessage(tlRequest)
	if err != nil {
		return err
	}

	fmt.Printf("Unsubscribed from BestFive tokens: %s, MarketSegmentId: %d\n", token, marketSegmentID)
	return nil
}

// SendMessage sends a message to the WebSocket server
func (tw *ODINMarketFeedClient) SendMessage(message string) error {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.conn == nil {
		return fmt.Errorf("WebSocket is not connected")
	}

	fmt.Println("Sending Message:", message)
	packet, err := tw.fragHandler.FragmentData([]byte(message))
	if err != nil {
		return err
	}

	return tw.conn.WriteMessage(websocket.BinaryMessage, packet)
}

func (tw *ODINMarketFeedClient) receiveMessages() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in receiveMessages:", r)
		}
	}()

	for {
		_, message, err := tw.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("Error in receive loop: %v\n", err)
			}
			if tw.OnError != nil {
				tw.OnError(err.Error())
			}
			break
		}

		tw.responseReceived(message)
	}
}
func (tw *ODINMarketFeedClient) responseReceived(data []byte) {

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Error in responseReceived:", r)
		}
	}()

	arrData, err := tw.fragHandler.Defragment(data)
	if err != nil {
		fmt.Printf("Error defragmenting data: %v\n", err)
		return
	}

	for i := 0; i < len(arrData); i++ {
		strMsg := string(arrData[i])

		if strings.Contains(strMsg, "|50=") {
			data := arrData[i]
			dataIndex := strings.Index(strMsg, "|50=") + 4
			strNewMsg := strMsg[:strings.Index(strMsg, "|50=")+1]

			// Market Segment ID
			mktSegID := binary.LittleEndian.Uint32(data[dataIndex : dataIndex+4])
			strNewMsg += "1=" + strconv.FormatUint(uint64(mktSegID), 10) + "|"

			// Token
			token := binary.LittleEndian.Uint32(data[dataIndex+4 : dataIndex+8])
			strNewMsg += "7=" + strconv.FormatUint(uint64(token), 10) + "|"

			// Last Update Time (LUT)
			lutSeconds := int32(binary.LittleEndian.Uint32(data[dataIndex+8 : dataIndex+12]))
			lutDate := tw.dteNSE.Add(time.Duration(lutSeconds) * time.Second)
			lut := lutDate.Format("2006-01-02 150405")
			strNewMsg += "74=" + lut + "|"

			// Last Trade Time (LTT)
			lttSeconds := int32(binary.LittleEndian.Uint32(data[dataIndex+12 : dataIndex+16]))
			lttDate := tw.dteNSE.Add(time.Duration(lttSeconds) * time.Second)
			ltt := lttDate.Format("2006-01-02 150405")
			strNewMsg += "73=" + ltt + "|"

			// Last Traded Price (LTP)
			ltp := binary.LittleEndian.Uint32(data[dataIndex+16 : dataIndex+20])
			strNewMsg += "8=" + strconv.FormatUint(uint64(ltp), 10) + "|"

			// Buy Quantity (BQty)
			bQty := binary.LittleEndian.Uint32(data[dataIndex+20 : dataIndex+24])
			strNewMsg += "2=" + strconv.FormatUint(uint64(bQty), 10) + "|"

			// Buy Price (BPrice)
			bPrice := binary.LittleEndian.Uint32(data[dataIndex+24 : dataIndex+28])
			strNewMsg += "3=" + strconv.FormatUint(uint64(bPrice), 10) + "|"

			// Sell Quantity (SQty) - but using BQty (bug in original code)
			sQty := binary.LittleEndian.Uint32(data[dataIndex+28 : dataIndex+32])
			strNewMsg += "5=" + strconv.FormatUint(uint64(sQty), 10) + "|" // Note: Original code uses BQty here (appears to be a bug)

			// Sell Price (SPrice) - but using BPrice (bug in original code)
			sPrice := binary.LittleEndian.Uint32(data[dataIndex+32 : dataIndex+36])
			strNewMsg += "6=" + strconv.FormatUint(uint64(sPrice), 10) + "|" // Note: Original code uses BPrice here (appears to be a bug)

			// Open Price (OPrice) - but using BQty (bug in original code)
			oPrice := binary.LittleEndian.Uint32(data[dataIndex+36 : dataIndex+40])
			strNewMsg += "75=" + strconv.FormatUint(uint64(oPrice), 10) + "|" // Note: Original code uses BQty here (appears to be a bug)

			// High Price (HPrice) - but using BPrice (bug in original code)
			hPrice := binary.LittleEndian.Uint32(data[dataIndex+40 : dataIndex+44])
			strNewMsg += "77=" + strconv.FormatUint(uint64(hPrice), 10) + "|" // Note: Original code uses BPrice here (appears to be a bug)

			// Low Price (LPrice) - but using BQty (bug in original code)
			lPrice := binary.LittleEndian.Uint32(data[dataIndex+44 : dataIndex+48])
			strNewMsg += "78=" + strconv.FormatUint(uint64(lPrice), 10) + "|" // Note: Original code uses BQty here (appears to be a bug)

			// Close Price (CPrice) - but using BPrice (bug in original code)
			cPrice := binary.LittleEndian.Uint32(data[dataIndex+48 : dataIndex+52])
			strNewMsg += "76=" + strconv.FormatUint(uint64(cPrice), 10) + "|" // Note: Original code uses BPrice here (appears to be a bug)

			// Decimal Locator
			decLocator := binary.LittleEndian.Uint32(data[dataIndex+52 : dataIndex+56])
			strNewMsg += "399=" + strconv.FormatUint(uint64(decLocator), 10) + "|"

			// Previous Close Price
			prvClosePrice := binary.LittleEndian.Uint32(data[dataIndex+56 : dataIndex+60])
			strNewMsg += "250=" + strconv.FormatUint(uint64(prvClosePrice), 10) + "|"

			// Indicative Close Price
			indicativeClosePrice := binary.LittleEndian.Uint32(data[dataIndex+60 : dataIndex+64])
			strNewMsg += "88=" + strconv.FormatUint(uint64(indicativeClosePrice), 10) + "|"

			strMsg = strNewMsg
		}

		if tw.OnMessage != nil {
			tw.OnMessage(strMsg)
		}
	}

}

func parseData(data string) ([]string, error) {
	messages := make([]string, 0)

	for len(data) > 6 {
		// Ensure we have at least 6 characters
		if len(data) < 6 {
			break
		}

		// Extract length from characters at index 1-5 (5 characters)
		lengthStr := data[1:6]
		messageLength, err := strconv.Atoi(lengthStr)
		if err != nil {
			return messages, fmt.Errorf("failed to parse message length '%s': %w", lengthStr, err)
		}

		// Check if we have a complete message
		totalLength := 6 + messageLength

		if len(data) < totalLength {
			// Not enough data yet, wait for more
			break
		}

		// Extract the message data (skip first 6 characters: # + 5-digit length)
		messageData := data[6:totalLength]
		messages = append(messages, messageData)

		// Remove processed message from buffer
		data = data[totalLength:]
	}

	return messages, nil
}

// splitByFIXStart splits the input by the delimiter and keeps the delimiter at start of each message
func splitByFIXStart(input, delimiter string) []string {
	var result []string

	// Find indices where delimiter starts
	indices := []int{}
	for i := 0; i < len(input); i++ {
		if len(input[i:]) >= len(delimiter) && input[i:i+len(delimiter)] == delimiter {
			indices = append(indices, i)
		}
	}

	// Split the string at those indices
	for i, startIdx := range indices {
		var endIdx int
		if i+1 < len(indices) {
			endIdx = indices[i+1]
		} else {
			endIdx = len(input)
		}
		result = append(result, input[startIdx:endIdx])
	}

	return result
}

// Dispose releases resources
func (tw *ODINMarketFeedClient) Dispose() {
	if !tw.isDisposed {
		if tw.conn != nil {
			tw.conn.Close()
		}
		tw.isDisposed = true
	}
}

// Example usage
/*
func main() {
	client := NewODINMarketFeedClient()

	client.OnOpen = func() {
		fmt.Println("WebSocket opened")
	}

	client.OnMessage = func(message string) {
		fmt.Println("Received:", message)
	}

	client.OnError = func(err string) {
		fmt.Println("Error:", err)
	}

	client.OnClose = func(code int, reason string) {
		fmt.Printf("Closed: %d - %s\n", code, reason)
	}

	err := client.Connect("your-host.com", 8080, true, "your_user_id")
	if err != nil {
		panic(err)
	}

	// Keep connection alive
	time.Sleep(60 * time.Second)

	client.Disconnect()
}
*/
