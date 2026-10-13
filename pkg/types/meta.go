/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	timeLayout = "2006-01-02 15:04:05.999999999"

	MsgData   = '1'
	MsgResize = '2'

	// WebSocket 保活 ping 的发送间隔与单次写入超时
	wsHeartbeatInterval = 30 * time.Second
	wsHeartbeatTimeout  = 10 * time.Second
)

func (c *Cluster) SetId(i int64) {
	c.Id = i
}

func (o *KubeObject) SetReplicaSets(replicaSets []appv1.ReplicaSet) {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.ReplicaSets = replicaSets
}

func (o *KubeObject) GetReplicaSets() []appv1.ReplicaSet {
	o.lock.Lock()
	defer o.lock.Unlock()

	return o.ReplicaSets
}

func (o *KubeObject) SetPods(pods []v1.Pod) {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.Pods = pods
}

func (o *KubeObject) GetPods() []v1.Pod {
	o.lock.Lock()
	defer o.lock.Unlock()

	return o.Pods
}

func FormatTime(GmtCreate time.Time, GmtModified time.Time) TimeSpec {
	return TimeSpec{
		GmtCreate:   GmtCreate.Format(timeLayout),
		GmtModified: GmtModified.Format(timeLayout),
	}
}

// NewTerminalSession 该方法用于升级 http 协议至 websocket，并new一个 TerminalSession 类型的对象返回
func NewTerminalSession(w http.ResponseWriter, r *http.Request) (*TerminalSession, error) {
	// 初始化 Upgrader 类型的对象，用于http协议升级为 websocket 协议
	upgrader := &websocket.Upgrader{
		HandshakeTimeout: time.Second * 2,
		// 检测请求来源
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols: []string{r.Header.Get("Sec-WebSocket-Protocol")},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	session := &TerminalSession{
		wsConn:   conn,
		sizeChan: make(chan remotecommand.TerminalSize),
		doneChan: make(chan struct{}),
	}

	return session, nil
}

// 用于读取web端的输入，接收web端输入的指令内容
func (t *TerminalSession) Read(p []byte) (int, error) {
	_, message, err := t.wsConn.ReadMessage()
	if err != nil {
		return copy(p, "\u0004"), err
	}
	// 反序列化
	var msg TerminalMessage
	if err = json.Unmarshal(message, &msg); err != nil {
		return copy(p, "\u0004"), err
	}
	// 逻辑判断
	switch msg.Operation {
	// 如果是标准输入
	case "stdin":
		return copy(p, msg.Data), nil
	// 窗口调整大小
	case "resize":
		t.sizeChan <- remotecommand.TerminalSize{Width: msg.Cols, Height: msg.Rows}
		return 0, nil
	// ping	无内容交互
	case "ping":
		return 0, nil
	default:
		return copy(p, "\u0004"), fmt.Errorf("unknown message type")
	}
}

// 写数据的方法，拿到 api-server 的返回内容，向web端输出
func (t *TerminalSession) Write(p []byte) (int, error) {
	msg, err := json.Marshal(TerminalMessage{
		Operation: "stdout",
		Data:      string(p),
	})
	if err != nil {
		return 0, err
	}
	if err = t.wsConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Done 标记关闭doneChan,关闭后触发退出终端
func (t *TerminalSession) Done() {
	close(t.doneChan)
}

// Close 用于关闭websocket连接
func (t *TerminalSession) Close() error {
	return t.wsConn.Close()
}

// StartHeartbeat 启动 WebSocket 保活心跳，返回幂等的停止函数。
// 由服务端周期下发 ping，避免空闲时被 nginx 的 proxy_read_timeout 断开。
func (t *TerminalSession) StartHeartbeat() func() {
	done := make(chan struct{})
	var once sync.Once

	go func() {
		ticker := time.NewTicker(wsHeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := t.wsConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsHeartbeatTimeout)); err != nil {
					// 连接已不可写，无需继续心跳，退出即可
					klog.Errorf("failed to send websocket heartbeat ping: %v", err)
					return
				}
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(done)
		})
	}
}

// Next 获取web端是否resize,以及是否退出终端
func (t *TerminalSession) Next() *remotecommand.TerminalSize {
	select {
	case size := <-t.sizeChan:
		return &size
	case <-t.doneChan:
		return nil
	}
}

func NewTurn(wsConn *websocket.Conn, sshClient *ssh.Client) (*Turn, error) {
	session, err := sshClient.NewSession()
	if err != nil {
		return nil, err
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}

	turn := &Turn{StdinPipe: stdinPipe, Session: session, WsConn: wsConn}
	session.Stdout = turn
	session.Stderr = turn

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // disable echo
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	if err = session.RequestPty("xterm", 150, 30, modes); err != nil {
		return nil, err
	}
	if err = session.Shell(); err != nil {
		return nil, err
	}

	return turn, nil
}

func (t *Turn) Write(p []byte) (n int, err error) {
	writer, err := t.WsConn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	defer writer.Close()

	return writer.Write(p)
}

func (t *Turn) Close() error {
	if t.Session != nil {
		t.Session.Close()
	}
	return t.WsConn.Close()
}

func (t *Turn) Read(p []byte) (n int, err error) {
	for {
		msgType, reader, err := t.WsConn.NextReader()
		if err != nil {
			return 0, err
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		return reader.Read(p)
	}
}

func (t *Turn) StartLoopRead(ctx context.Context, wg *sync.WaitGroup, logBuff *bytes.Buffer) {
	defer wg.Done()
	err := t.loopRead(logBuff, ctx)
	if err != nil {
		klog.Errorf("LoopRead exit, err:%s", err)
	}
}

func (t *Turn) loopRead(logBuff *bytes.Buffer, context context.Context) error {
	for {
		select {
		case <-context.Done():
			return fmt.Errorf("LoopRead exit")
		default:
			_, wsData, err := t.WsConn.ReadMessage()
			if err != nil {
				return fmt.Errorf("reading webSocket message err:%s", err)
			}
			body := decode(wsData[1:])

			switch wsData[0] {
			case MsgResize:
				if err := t.resizeDo(body); err != nil {
					return err
				}
			case MsgData:
				if err := t.dataDo(body, logBuff); err != nil {
					return err
				}
			}
		}
	}
}

func (t *Turn) dataDo(body []byte, logBuff *bytes.Buffer) error {
	if _, err := t.StdinPipe.Write(body); err != nil {
		return fmt.Errorf("StdinPipe write err:%s", err)
	}

	if _, err := logBuff.Write(body); err != nil {
		return fmt.Errorf("logBuff write err:%s", err)
	}
	return nil
}

type Resize struct {
	Columns int
	Rows    int
}

func (t *Turn) resizeDo(body []byte) error {
	var args Resize
	err := json.Unmarshal(body, &args)
	if err != nil {
		return fmt.Errorf("ssh pty resize windows err:%s", err)
	}

	if args.Columns > 0 && args.Rows > 0 {
		if err := t.Session.WindowChange(args.Rows, args.Columns); err != nil {
			return fmt.Errorf("ssh pty resize windows err:%s", err)
		}
	}
	return nil
}

func (t *Turn) sessionWait() error {
	if err := t.Session.Wait(); err != nil {
		return err
	}
	return nil
}

func (t *Turn) StartSessionWait(wg *sync.WaitGroup) {
	defer wg.Done()
	err := t.sessionWait()
	if err != nil {
		klog.Errorf("SessionWait exit, err:%s", err)
	}
}

// StartHeartbeat 与 TerminalSession.StartHeartbeat 同义，用于 Node / 集群 SSH 会话。
func (t *Turn) StartHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(wsHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := t.WsConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsHeartbeatTimeout)); err != nil {
				klog.Errorf("failed to send websocket heartbeat ping: %v", err)
				return
			}
		}
	}
}

func decode(p []byte) []byte {
	decodeString, _ := base64.StdEncoding.DecodeString(string(p))
	return decodeString
}

func (a *PlanNodeAuth) Marshal() (string, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *PlanNodeAuth) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), a); err != nil {
		return err
	}
	return nil
}

func (ks *KubernetesSpec) Marshal() (string, error) {
	data, err := json.Marshal(ks)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (ks *KubernetesSpec) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), ks); err != nil {
		return err
	}
	return nil
}

func (ns *NetworkSpec) Marshal() (string, error) {
	data, err := json.Marshal(ns)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (ns *NetworkSpec) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), ns); err != nil {
		return err
	}
	return nil
}

func (rs *RuntimeSpec) Marshal() (string, error) {
	data, err := json.Marshal(rs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (rs *RuntimeSpec) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), rs); err != nil {
		return err
	}
	return nil
}

func (cs ComponentSpec) Marshal() (string, error) {
	data, err := json.Marshal(cs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (cs *ComponentSpec) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), cs); err != nil {
		return err
	}
	return nil
}

func (rs *RuntimeSpec) IsDocker() bool {
	return rs.Runtime == string(model.DockerCRI)
}

func (rs *RuntimeSpec) IsContainerd() bool {
	return rs.Runtime == string(model.ContainerdCRI)
}

func (p PageRequest) IsPaged() bool {
	return p.Page != 0 && p.Limit != 0
}

func (p PageRequest) Offset(total int) (int, int, error) {
	offset := (p.Page - 1) * p.Limit
	if offset > total {
		return 0, 0, fmt.Errorf("invaild offset")
	}

	end := offset + p.Limit
	if end > total {
		end = total
	}

	return offset, end, nil
}

func (node *KubeNode) Marshal() (string, error) {
	data, err := json.Marshal(node)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (node *KubeNode) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), node); err != nil {
		return err
	}
	return nil
}

func (c *DatasourceConfig) Marshal() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *DatasourceConfig) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), c); err != nil {
		return err
	}
	return nil
}

// Clean 移除不必要的配置，防止持久化的时候存储多余配置
func (c *DatasourceConfig) Clean(t model.DatasourceType, subType model.DatasourceSubType) {
	// 如果是告警，则清空日志相关配置
	if t == model.DatasourceTypeAlert {
		if c.Log != nil {
			c.Log = nil
		}
		c.Nacos = nil
	}
	if t == model.DatasourceTypeLog {
		if c.Alert != nil {
			c.Alert = nil
		}
	}
	// 中间件保留 log（Nacos 鉴权账号）与 headers，仅清空告警配置
	if t == model.DatasourceTypeMiddleware {
		c.Alert = nil
	}
	// Redis 与日志/告警配置互斥，只保留自身配置；redis 可搭配缓存（存量）或中间件类型，故以 sub_type 判定
	if t == model.DatasourceTypeRedis || subType == model.DatasourceSubTypeRedis {
		c.Log = nil
		c.Alert = nil
		c.Headers = nil
		c.Nacos = nil
	} else {
		c.Redis = nil
	}
	if subType != model.DatasourceSubTypeMySQL {
		c.Mysql = nil
	}
	if subType != model.DatasourceSubTypePostgres {
		c.Postgres = nil
	}
	if subType != model.DatasourceSubTypeStorage {
		c.Storage = nil
	}
}
