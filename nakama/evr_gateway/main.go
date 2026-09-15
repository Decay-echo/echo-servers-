package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ServerInfo holds registered Echo VR server metadata
type ServerInfo struct {
	ServerID      string    `json:"server_id"`
	Region        string    `json:"region"`
	InternalIP    string    `json:"internal_ip"`
	ExternalIP    string    `json:"external_ip"`
	Port          uint32    `json:"port"`
	Version       string    `json:"version"`
	DiscordID     string    `json:"discord_id"`
	Password      string    `json:"password"`
	Guilds        string    `json:"guilds"`
	Regions       string    `json:"regions"`
	PriorityModes string    `json:"priority_modes"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	UUID          string    `json:"uuid"`
	Endpoint      string    `json:"endpoint"`
}

type Gateway struct {
	mu        sync.RWMutex
	servers   map[string]*ServerInfo
	regions   map[string][]*ServerInfo
	passwords map[string]string // discordid -> password for auth
	upgrader  websocket.Upgrader
	logger    *log.Logger
}

func NewGateway() *Gateway {
	return &Gateway{
		servers:   make(map[string]*ServerInfo),
		regions:   make(map[string][]*ServerInfo),
		passwords: make(map[string]string),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			ReadBufferSize:  1024 * 64,
			WriteBufferSize: 1024 * 64,
		},
		logger: log.New(log.Writer(), "[EVR-GW] ", log.LstdFlags|log.Lshortfile),
	}
}

func (g *Gateway) RegisterServer(info *ServerInfo) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if info.UUID == "" {
		info.UUID = "B9FF626F-CC9B-4680-8E10-AE0BFBEA22FB"
	}

	info.LastHeartbeat = time.Now()

	if oldServer, exists := g.servers[info.ServerID]; exists && oldServer.Region != info.Region {
		g.removeFromRegion(oldServer.Region, info.ServerID)
	}

	g.servers[info.ServerID] = info
	g.regions[info.Region] = append(g.regions[info.Region], info)

	g.logger.Printf("Server registered: %s (region: %s, port: %d, UUID: %s)\n",
		info.ServerID, info.Region, info.Port, info.UUID)
}

func (g *Gateway) removeFromRegion(region, serverID string) {
	if servers, exists := g.regions[region]; exists {
		for i, s := range servers {
			if s.ServerID == serverID {
				g.regions[region] = append(servers[:i], servers[i+1:]...)
				break
			}
		}
		if len(g.regions[region]) == 0 {
			delete(g.regions, region)
		}
	}
}

func (g *Gateway) GetServersByRegion(region string) []*ServerInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.regions[region]
}

func (g *Gateway) HandleWebSocket(w http.ResponseWriter, r *http.Request, endpoint string) {
	// Parse URL parameters
	query := r.URL.Query()
	discordID := query.Get("discordid")
	password := query.Get("password")
	guilds := query.Get("guilds")
	regions := query.Get("regions")
	priorityModes := query.Get("tags")

	// Authenticate if discordid provided
	if discordID != "" {
		if endpoint == "serverdb" && password != "" {
			// Store or validate password
			g.mu.Lock()
			if storedPwd, exists := g.passwords[discordID]; exists {
				// Password already set, validate
				if storedPwd != password {
					g.mu.Unlock()
					g.logger.Printf("%s auth failed for %s: invalid password\n", endpoint, discordID)
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			} else {
				// First connection, store password
				g.passwords[discordID] = password
			}
			g.mu.Unlock()
		}
		g.logger.Printf("%s auth: discordid=%s\n", endpoint, discordID)
	}

	conn, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.logger.Printf("%s upgrade error: %v\n", endpoint, err)
		return
	}
	defer conn.Close()

	// Add default region if not specified
	if regions == "" {
		regions = "default"
	} else if !contains(regions, "default") {
		regions = regions + ",default"
	}

	g.logger.Printf("%s connected (endpoint=%s, discordid=%s, regions=%s, guilds=%s)\n",
		endpoint, endpoint, discordID, regions, guilds)

	conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
		return nil
	})

	// Send initial ping to establish connection
	if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
		g.logger.Printf("%s initial ping error: %v\n", endpoint, err)
		return
	}

	// Keep connection alive - read and log messages
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				g.logger.Printf("%s error: %v\n", endpoint, err)
			}
			break
		}

		// Log message type and data in hex
		g.logger.Printf("%s message received: type=%d, size=%d bytes, data=%x\n", 
			endpoint, messageType, len(data), data)

		// Parse as JSON if possible for server registration
		if endpoint == "serverdb" {
			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err == nil {
				if serverID, ok := msg["server_id"].(string); ok {
					info := &ServerInfo{
						ServerID:      serverID,
						Region:        regions,
						Version:       "1.0",
						DiscordID:     discordID,
						Password:      password,
						Guilds:        guilds,
						Regions:       regions,
						PriorityModes: priorityModes,
						Endpoint:      endpoint,
					}
					if internalIP, ok := msg["internal_ip"].(string); ok {
						info.InternalIP = internalIP
					}
					if externalIP, ok := msg["external_ip"].(string); ok {
						info.ExternalIP = externalIP
					}
					if port, ok := msg["port"].(float64); ok {
						info.Port = uint32(port)
					}
					g.RegisterServer(info)
				}
			}
		}

		// Echo message back immediately
		if err := conn.WriteMessage(messageType, data); err != nil {
			g.logger.Printf("%s write error: %v\n", endpoint, err)
			break
		}
	}
}

func contains(s, substr string) bool {
	for _, part := range strings.Split(s, ",") {
		if strings.TrimSpace(part) == substr {
			return true
		}
	}
	return false
}

func (g *Gateway) HandleAPIServers(w http.ResponseWriter, r *http.Request) {
	region := r.URL.Query().Get("region")
	if region == "" {
		region = "us-oklahoma-kingfisher"
	}

	servers := g.GetServersByRegion(region)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"region":  region,
		"servers": servers,
	})
}

func (g *Gateway) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	gw := NewGateway()

	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		gw.HandleWebSocket(w, r, "config")
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		gw.HandleWebSocket(w, r, "login")
	})

	http.HandleFunc("/matching", func(w http.ResponseWriter, r *http.Request) {
		gw.HandleWebSocket(w, r, "matching")
	})

	http.HandleFunc("/serverdb", func(w http.ResponseWriter, r *http.Request) {
		gw.HandleWebSocket(w, r, "serverdb")
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		gw.HandleWebSocket(w, r, "ws")
	})

	// Echo VR protocol endpoint
	http.HandleFunc("/rad/rad15_live", func(w http.ResponseWriter, r *http.Request) {
		gw.HandleWebSocket(w, r, "rad15_live")
	})

	http.HandleFunc("/api/servers", gw.HandleAPIServers)
	http.HandleFunc("/health", gw.HandleHealthCheck)

	gw.logger.Println("EVR Gateway starting on :7360")
	if err := http.ListenAndServe(":7360", nil); err != nil {
		gw.logger.Fatalf("Server error: %v\n", err)
	}
}
