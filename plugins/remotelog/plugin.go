// remotelog is a plugin that enables log messages being sent via UDP to a central ELK stack for debugging.
// It is disabled by default and when enabled, additionally, logger.disableEvents=false in config.json needs to be set.
// The destination can be set via logger.remotelog.serverAddress.
// All events according to logger.level in config.json are sent.
package remotelog

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/cli"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

	"gopkg.in/src-d/go-git.v4"
)

type logMessage struct {
	Version   string    `json:"version"`
	GitHead   string    `json:"gitHead,omitempty"`
	GitBranch string    `json:"gitBranch,omitempty"`
	NodeId    string    `json:"nodeId"`
	Level     string    `json:"level"`
	Name      string    `json:"name"`
	Msg       string    `json:"msg"`
	Timestamp time.Time `json:"timestamp"`
}

const (
	CFG_SERVER_ADDRESS = "logger.remotelog.serverAddress"
	CFG_DISABLE_EVENTS = "logger.disableEvents"
	PLUGIN_NAME        = "RemoteLog"
)

var (
	PLUGIN      = node.NewPlugin(PLUGIN_NAME, node.Disabled, configure, run)
	log         *logger.Logger
	conn        net.Conn
	myID        string
	myGitHead   string
	myGitBranch string
	workerPool  *workerpool.WorkerPool
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PLUGIN_NAME)

	if parameter.NodeConfig.GetBool(CFG_DISABLE_EVENTS) {
		log.Fatalf("%s in config.json needs to be false so that events can be captured!", CFG_DISABLE_EVENTS)
		return
	}

	c, err := net.Dial("udp", parameter.NodeConfig.GetString(CFG_SERVER_ADDRESS))
	if err != nil {
		log.Fatalf("Could not create UDP socket to '%s'. %v", parameter.NodeConfig.GetString(CFG_SERVER_ADDRESS), err)
		return
	}
	conn = c

	if local.GetInstance() != nil {
		myID = hex.EncodeToString(local.GetInstance().ID().Bytes())
	}

	getGitInfo()

	workerPool = workerpool.New(func(task workerpool.Task) {
		sendLogMsg(task.Param(0).(logger.Level), task.Param(1).(string), task.Param(2).(string))

		task.Return(nil)
	}, workerpool.WorkerCount(runtime.NumCPU()), workerpool.QueueSize(1000))
}

func run(plugin *node.Plugin) {
	logEvent := events.NewClosure(func(level logger.Level, name string, msg string) {
		workerPool.TrySubmit(level, name, msg)
	})

	daemon.BackgroundWorker(PLUGIN_NAME, func(shutdownSignal <-chan struct{}) {
		logger.Events.AnyMsg.Attach(logEvent)
		workerPool.Start()
		<-shutdownSignal
		log.Infof("Stopping %s ...", PLUGIN_NAME)
		logger.Events.AnyMsg.Detach(logEvent)
		workerPool.Stop()
		log.Infof("Stopping %s ... done", PLUGIN_NAME)
	}, shutdown.ShutdownPriorityRemoteLog)
}

func sendLogMsg(level logger.Level, name string, msg string) {
	m := logMessage{
		cli.AppVersion,
		myGitHead,
		myGitBranch,
		myID,
		level.CapitalString(),
		name,
		msg,
		time.Now(),
	}
	b, _ := json.Marshal(m)
	fmt.Fprint(conn, string(b))
}

func getGitInfo() {
	r, err := git.PlainOpen(getGitDir())
	if err != nil {
		log.Debug("Could not open Git repo.")
		return
	}

	// extract git branch and head
	if h, err := r.Head(); err == nil {
		myGitBranch = h.Name().String()
		myGitHead = h.Hash().String()
	}
}

func getGitDir() string {
	var gitDir string

	// this is valid when running an executable, when using "go run" this is a temp path
	if ex, err := os.Executable(); err == nil {
		temp := filepath.Join(filepath.Dir(ex), ".git")
		if _, err := os.Stat(temp); err == nil {
			gitDir = temp
		}
	}

	// when running "go run" from the same directory
	if gitDir == "" {
		if wd, err := os.Getwd(); err == nil {
			temp := filepath.Join(wd, ".git")
			if _, err := os.Stat(temp); err == nil {
				gitDir = temp
			}
		}
	}

	return gitDir
}
