package derpwhat

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"tailscale.com/derp"

	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/mdp/qrterminal"
	_ "modernc.org/sqlite"
)

type DerpWhat struct {
	s   *derp.Server
	dir string // in which to store what.db
	c   *whatsmeow.Client

	mu sync.Mutex
	// active conversations with peers
	conns map[types.JID]*dmconn
}

func New(s *derp.Server, dir string) (*DerpWhat, error) {
	dw := &DerpWhat{s: s, dir: dir}

	return dw, nil
}

func (dw *DerpWhat) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Presence:
		// TODO: route presence change to active connection
	case *events.Message:
		dw.handleMessage(v)
	}
}

const introMagic = "☁️tailscale"

func (dw *DerpWhat) handleMessage(m *events.Message) {
	if !m.IsViewOnce {
		return
	}

	vom := m.Message.GetViewOnceMessage()
	s := vom.GetMessage().GetConversation()

	jid := m.Info.Sender
	if c := dw.conns[jid]; c != nil {
		c.rxQ <- []byte(s)
		return
	}

	if s != introMagic {
		return
	}

	dw.newConnection(jid)

	fmt.Printf("Got view once intro message: %+v\n", m)
	return
}

func (dw *DerpWhat) newConnection(jid types.JID) {
	conn := &dmconn{
		jid: jid,
	}

	dw.mu.Lock()
	dw.conns[jid] = conn
	dw.mu.Unlock()

	dw.s.Accept(
		context.TODO(),
		conn,
		conn.brw(),
		jid.String(), // TODO: is the jid ok as a remoteaddr?
	)
	// TODO: fill in the remainder of presence usage
	dw.c.SubscribePresence(jid)
}

func (dw *DerpWhat) Start() error {
	dbLog := waLog.Stdout("Database", "INFO", true)

	container, err := sqlstore.New("sqlite", fmt.Sprintf("file:%s/what.db?_foreign_keys=on", dw.dir), dbLog)
	if err != nil {
		return err
	}
	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		return err
	}
	clientLog := waLog.Stdout("Client", "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(func(evt interface{}) {
		dw.eventHandler(evt)
	})
	dw.c = client

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			return err
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// Render the QR code here
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			return err
		}
	}

	return nil
	// client.Disconnect()
}
