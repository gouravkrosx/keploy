package connection

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	structs2 "go.keploy.io/server/pkg/hooks/structs"
	"go.uber.org/zap"
	// "log"
)

const (
	maxBufferSize = 16 * 1024 * 1024 // 16MB
)

type Tracker struct {
	connID         structs2.ConnID
	addr           structs2.SockAddrIn
	openTimestamp  uint64
	closeTimestamp uint64

	// Indicates the tracker stopped tracking due to closing the session.
	lastActivityTimestamp uint64

	// Queues to handle multiple ingress traffic on the same connection (keep-alive)
	totalSentBytesQueue   []uint64
	totalRecvBytesQueue   []uint64
	currentSentBytesQueue []uint64
	currentRecvBytesQueue []uint64
	currentSentBufQueue   [][]byte
	currentRecvBufQueue   [][]byte

	// Individual parameters to store current request and response data
	sentBytes uint64
	recvBytes uint64
	SentBuf   []byte
	RecvBuf   []byte

	// Additional fields to know when to capture request or response info
	gotResponseDataEvent  bool
	gotRequestDataEvent   bool
	recordTestCountAtomic int32
	firstRequest          bool

	mutex  sync.RWMutex
	logger *zap.Logger
}

func NewTracker(connID structs2.ConnID, logger *zap.Logger) *Tracker {
	return &Tracker{
		connID:                connID,
		RecvBuf:               []byte{},
		SentBuf:               []byte{},
		totalSentBytesQueue:   []uint64{},
		totalRecvBytesQueue:   []uint64{},
		currentSentBytesQueue: []uint64{},
		currentRecvBytesQueue: []uint64{},
		currentSentBufQueue:   [][]byte{},
		currentRecvBufQueue:   [][]byte{},
		mutex:                 sync.RWMutex{},
		logger:                logger,
		firstRequest:          true,
	}
}

func (conn *Tracker) ToBytes() ([]byte, []byte) {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()
	return conn.RecvBuf, conn.SentBuf
}

func (conn *Tracker) IsInactive(duration time.Duration) bool {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()
	return uint64(time.Now().UnixNano())-conn.lastActivityTimestamp > uint64(duration.Nanoseconds())
}

func (conn *Tracker) incRecordTestCount() {
	atomic.AddInt32(&conn.recordTestCountAtomic, 1)
}

func (conn *Tracker) decRecordTestCount() {
	atomic.AddInt32(&conn.recordTestCountAtomic, -1)
}

func (conn *Tracker) IsComplete() (bool, []byte, []byte) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	// Get the current timestamp in nanoseconds.
	currentTimestamp := uint64(time.Now().UnixNano())

	// Calculate the time elapsed since the last activity in nanoseconds.
	elapsedTime := currentTimestamp - conn.lastActivityTimestamp

	//Caveat: Added a timeout of 7 seconds, after this duration we assume that all the response data events would have come.
	// This will ensure that we capture the requests responses where Connection:keep-alive is enabled.

	recordTraffic := false

	requestBuf, responseBuf := []byte{}, []byte{}

	//if recordTestCountAtomic > 0, it means that we have num(recordTestCountAtomic) of request and response present in the queues to record.
	if conn.recordTestCountAtomic > 0 {
		if (len(conn.currentRecvBytesQueue) > 0 && len(conn.totalRecvBytesQueue) > 0) &&
			(len(conn.currentSentBytesQueue) > 0 && len(conn.totalSentBytesQueue) > 0) {
			validReq, validRes := false, false

			expectedRecvBytes := conn.currentRecvBytesQueue[0]
			actualRecvBytes := conn.totalRecvBytesQueue[0]

			//popping out the current request info
			conn.currentRecvBytesQueue = conn.currentRecvBytesQueue[1:]
			conn.totalRecvBytesQueue = conn.totalRecvBytesQueue[1:]

			if conn.verifyRequestData(expectedRecvBytes, actualRecvBytes) {
				validReq = true
			} else {
				conn.logger.Debug("Malformed request", zap.Any("ExpectedRecvBytes", expectedRecvBytes), zap.Any("ActualRecvBytes", actualRecvBytes))
				recordTraffic = false
			}

			expectedSentBytes := conn.currentSentBytesQueue[0]
			actualSentBytes := conn.totalSentBytesQueue[0]

			//popping out the current response info
			conn.currentSentBytesQueue = conn.currentSentBytesQueue[1:]
			conn.totalSentBytesQueue = conn.totalSentBytesQueue[1:]

			if conn.verifyResponseData(expectedSentBytes, actualSentBytes) {
				validRes = true
			} else {
				conn.logger.Debug("Malformed response", zap.Any("ExpectedSentBytes", expectedSentBytes), zap.Any("ActualSentBytes", actualSentBytes))
				recordTraffic = false
			}

			if len(conn.currentRecvBufQueue) > 0 && len(conn.currentSentBufQueue) > 0 { //validated request, response
				requestBuf = conn.currentRecvBufQueue[0]
				responseBuf = conn.currentSentBufQueue[0]

				//popping out the current request & response data
				conn.currentRecvBufQueue = conn.currentRecvBufQueue[1:]
				conn.currentSentBufQueue = conn.currentSentBufQueue[1:]
			} else {
				conn.logger.Debug("no data buffer for request or response", zap.Any("Length of RecvBufQueue", len(conn.currentRecvBufQueue)), zap.Any("Length of SentBufQueue", len(conn.currentSentBufQueue)))
				recordTraffic = false
			}

			recordTraffic = validReq && validRes
		} else {
			conn.logger.Error("malformed request or response")
			recordTraffic = false
		}

		conn.logger.Debug(fmt.Sprintf("recording traffic after verifying the request and reponse data:%v", recordTraffic))

		// // decrease the recordtestCount
		conn.decRecordTestCount()
		conn.logger.Debug("verified recording", zap.Any("recordTraffic", recordTraffic))
	} else if conn.gotResponseDataEvent && elapsedTime >= uint64(time.Second*4) { // Check if 4 second has passed since the last activity.
		conn.logger.Debug("might be last request on the connection")

		if len(conn.currentRecvBytesQueue) > 0 && len(conn.totalRecvBytesQueue) > 0 {

			expectedRecvBytes := conn.currentRecvBytesQueue[0]
			actualRecvBytes := conn.totalRecvBytesQueue[0]

			//popping out the current request info
			conn.currentRecvBytesQueue = conn.currentRecvBytesQueue[1:]
			conn.totalRecvBytesQueue = conn.totalRecvBytesQueue[1:]

			if conn.verifyRequestData(expectedRecvBytes, actualRecvBytes) {
				recordTraffic = true
			} else {
				conn.logger.Debug("Malformed request", zap.Any("ExpectedRecvBytes", expectedRecvBytes), zap.Any("ActualRecvBytes", actualRecvBytes))
				recordTraffic = false
			}

			if len(conn.currentRecvBufQueue) > 0 { //validated request, invalided response
				requestBuf = conn.currentRecvBufQueue[0]
				//popping out the current request data
				conn.currentRecvBufQueue = conn.currentRecvBufQueue[1:]

				responseBuf = conn.SentBuf
			} else {
				conn.logger.Debug("no data buffer for request", zap.Any("Length of RecvBufQueue", len(conn.currentRecvBufQueue)))
				recordTraffic = false
			}

		} else {
			conn.logger.Error("malformed request")
			recordTraffic = false
		}

		conn.logger.Debug(fmt.Sprintf("recording traffic after verifying the request data (but not response data):%v", recordTraffic))
		//treat immediate next request as first request (4 seconds after last activity)
		conn.resetConnection()

		conn.logger.Debug("unverified recording", zap.Any("recordTraffic", recordTraffic))
	}

	return recordTraffic, requestBuf, responseBuf
	// // Check if other conditions for completeness are met.
	// return conn.closeTimestamp != 0 &&
	// 	conn.totalReadBytes == conn.recvBytes &&
	// 	conn.totalWrittenBytes == conn.sentBytes
}

func (conn *Tracker) resetConnection() {
	conn.firstRequest = true
	conn.gotResponseDataEvent = false
	conn.gotRequestDataEvent = false
	conn.recvBytes = 0
	conn.sentBytes = 0
	conn.SentBuf = []byte{}
	conn.RecvBuf = []byte{}
}

func (conn *Tracker) verifyRequestData(expectedRecvBytes, actualRecvBytes uint64) bool {
	return (expectedRecvBytes == actualRecvBytes)
}

func (conn *Tracker) verifyResponseData(expectedSentBytes, actualSentBytes uint64) bool {
	return (expectedSentBytes == actualSentBytes)
}

// func (conn *Tracker) Malformed() bool {
// 	conn.mutex.RLock()
// 	defer conn.mutex.RUnlock()
// 	// conn.logger.Debug("data loss of ingress request message", zap.Any("bytes read in ebpf", conn.totalReadBytes), zap.Any("bytes recieved in userspace", conn.recvBytes))
// 	// conn.logger.Debug("data loss of ingress response message", zap.Any("bytes written in ebpf", conn.totalWrittenBytes), zap.Any("bytes sent to user", conn.sentBytes))
// 	// conn.logger.Debug("", zap.Any("Request buffer", string(conn.RecvBuf)))
// 	// conn.logger.Debug("", zap.Any("Response buffer", string(conn.SentBuf)))
// 	return conn.closeTimestamp != 0 &&
// 		conn.totalReadBytes != conn.recvBytes &&
// 		conn.totalWrittenBytes != conn.sentBytes
// }

func (conn *Tracker) AddDataEvent(event structs2.SocketDataEvent) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	conn.UpdateTimestamps()

	direction := ""
	if event.Direction == structs2.EgressTraffic {
		direction = "Egress"
	} else if event.Direction == structs2.IngressTraffic {
		direction = "Ingress"
	}

	conn.logger.Debug(fmt.Sprintf("Got a data event from eBPF, Direction:%v || current Event Size:%v || ConnectionID:%v\n", direction, event.MsgSize, event.ConnID))

	switch event.Direction {
	case structs2.EgressTraffic:
		// Assign the size of the message to the variable msgLengt
		msgLength := event.MsgSize
		// If the size of the message exceeds the maximum allowed size,
		// set msgLength to the maximum allowed size instead
		if event.MsgSize > structs2.EventBodyMaxSize {
			msgLength = structs2.EventBodyMaxSize
		}
		// Append the message (up to msgLength) to the connection's sent buffer
		conn.SentBuf = append(conn.SentBuf, event.Msg[:msgLength]...)
		conn.sentBytes += uint64(event.MsgSize)

		//Handling multiple request on same connection to support connection:keep-alive
		if conn.firstRequest || conn.gotRequestDataEvent {
			conn.currentRecvBytesQueue = append(conn.currentRecvBytesQueue, conn.recvBytes)
			conn.recvBytes = 0

			conn.currentRecvBufQueue = append(conn.currentRecvBufQueue, conn.RecvBuf)
			conn.RecvBuf = []byte{}

			conn.gotRequestDataEvent = false
			conn.gotResponseDataEvent = true

			conn.totalRecvBytesQueue = append(conn.totalRecvBytesQueue, uint64(event.ValidateReadBytes))
			conn.firstRequest = false
		}

	case structs2.IngressTraffic:
		// Assign the size of the message to the variable msgLength
		msgLength := event.MsgSize
		// If the size of the message exceeds the maximum allowed size,
		// set msgLength to the maximum allowed size instead
		if event.MsgSize > structs2.EventBodyMaxSize {
			msgLength = structs2.EventBodyMaxSize
		}
		// Append the message (up to msgLength) to the connection's receive buffer
		conn.RecvBuf = append(conn.RecvBuf, event.Msg[:msgLength]...)
		conn.recvBytes += uint64(event.MsgSize)

		//Handling multiple request on same connection to support connection:keep-alive
		if !conn.firstRequest || conn.gotResponseDataEvent {
			conn.currentSentBytesQueue = append(conn.currentSentBytesQueue, conn.sentBytes)
			conn.sentBytes = 0

			conn.currentSentBufQueue = append(conn.currentSentBufQueue, conn.SentBuf)
			conn.SentBuf = []byte{}

			conn.gotRequestDataEvent = true
			conn.gotResponseDataEvent = false

			conn.totalSentBytesQueue = append(conn.totalSentBytesQueue, uint64(event.ValidateWrittenBytes))

			//Record a test case for the current request/
			conn.incRecordTestCount()
		}

	default:
	}
}

func (conn *Tracker) AddOpenEvent(event structs2.SocketOpenEvent) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	conn.UpdateTimestamps()
	conn.addr = event.Addr
	if conn.openTimestamp != 0 && conn.openTimestamp != event.TimestampNano {
		conn.logger.Debug("Changed open info timestamp due to new request", zap.Any("from", conn.openTimestamp), zap.Any("to", event.TimestampNano))
	}
	// conn.logger.Debug("Got an open event from eBPF", zap.Any("File Descriptor", event.ConnID.FD))
	conn.openTimestamp = event.TimestampNano
}

func (conn *Tracker) AddCloseEvent(event structs2.SocketCloseEvent) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	conn.UpdateTimestamps()
	if conn.closeTimestamp != 0 && conn.closeTimestamp != event.TimestampNano {
		conn.logger.Debug("Changed close info timestamp due to new request", zap.Any("from", conn.closeTimestamp), zap.Any("to", event.TimestampNano))
	}
	conn.closeTimestamp = event.TimestampNano
	conn.logger.Debug(fmt.Sprintf("Got a close event from eBPF on connectionId:%v\n", event.ConnID))
}

func (conn *Tracker) UpdateTimestamps() {
	conn.lastActivityTimestamp = uint64(time.Now().UnixNano())
}
