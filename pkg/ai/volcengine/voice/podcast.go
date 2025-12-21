package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"

	"github.com/quka-ai/quka-ai/pkg/ai/volcengine/voice/protocols"
)

// https://www.volcengine.com/docs/6561/1668014?lang=zh#_4-%E8%B0%83%E7%94%A8%E7%A4%BA%E4%BE%8B

func VoiceToResourceId() string {
	return "volc.service_type.10050"
}

func NewPodCaster(appid, accessToken string) *PodCaster {
	return &PodCaster{
		appid:       appid,
		accessToken: accessToken,
	}
}

type PodCaster struct {
	appid       string
	accessToken string
}

type ProviderInfo struct {
	Name  string
	Model string
}

func (p *PodCaster) ProviderInfo() ProviderInfo {
	return ProviderInfo{
		Name:  "volcengine",
		Model: "API-websocket-v3",
	}
}

// ProgressCallback 进度回调函数类型
// 每当有新的 round 开始时调用，用于更新进度时间戳
type ProgressCallback func()

func (p *PodCaster) Gen(ctx context.Context, inputID, text string, flagUseHeadMusic, flagUseTailMusic bool, progressCallback ProgressCallback) (*Result, error) {
	flagEncoding := "mp3"
	flagOnlyNlpText := false
	flagSpeakerInfo := "{\"random_order\":false}"
	flagReturnAudioUrl := true
	flagAction := 0
	flagEndpoint := "wss://openspeech.bytedance.com/api/v3/sami/podcasttts"

	header := http.Header{}
	header.Set("X-Api-App-Id", p.appid)
	header.Set("X-Api-App-Key", p.appid)
	header.Set("X-Api-Access-Key", p.accessToken)
	header.Set("X-Api-Resource-Id", VoiceToResourceId())
	header.Set("X-Api-Connect-Id", uuid.New().String())

	var (
		isPodcastRoundEnd = true // 标志当前轮是否结束
		lastRoundID       = -1   // 上一轮的轮次ID
		taskID            = ""   // 任务ID
		retryNum          = 5    // 重试次数
		podcastTexts      = make([]map[string]interface{}, 0)

		metaInfo  MetaInfo
		usage     Usage
		durations decimal.Decimal
		result    Result
		err       error
	)

	for retryNum > 0 {
		err = func() error {
			// 建立WebSocket连接	client<----------->server
			conn, r, err := websocket.DefaultDialer.DialContext(ctx, flagEndpoint, header)
			if err != nil {
				return fmt.Errorf("Failed to connect service, %w", err)
			}

			result.RequestID = r.Header.Get("x-tt-logid")
			glog.Info("Connection established, Logid: ", r.Header.Get("x-tt-logid"))
			defer func() {
				// Cleanly close the connection by sending a close message
				err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					glog.Error(err)
				}
				err = conn.Close()
				if err != nil {
					glog.Error(err)
				}
			}()

			// flagSpeakerInfo 非空的时候需要转换成 json 对象
			var speakerInfo map[string]interface{}
			if flagSpeakerInfo != "" {
				if err := json.Unmarshal([]byte(flagSpeakerInfo), &speakerInfo); err != nil {
					glog.Exit(err)
				}
			}
			reqParams := map[string]interface{}{
				"input_id":       inputID,
				"input_text":     text,
				"action":         flagAction,
				"use_head_music": flagUseHeadMusic,
				"use_tail_music": flagUseTailMusic,
				"input_info": map[string]interface{}{
					"return_audio_url": flagReturnAudioUrl,
					"only_nlp_text":    flagOnlyNlpText,
				},
				"speaker_info": speakerInfo,
				"audio_config": map[string]interface{}{
					"format":      flagEncoding,
					"sample_rate": 24000,
					"speech_rate": 0,
				},
			}

			if !isPodcastRoundEnd {
				reqParams["retry_info"] = map[string]interface{}{
					"retry_task_id":          taskID,
					"last_finished_round_id": lastRoundID,
				}
			}
			// Start connection [event=1] -----------> server
			if err := protocols.StartConnection(conn); err != nil {
				return err
			}
			// Connection started [event=50] <---------- server
			_, err = protocols.WaitForEvent(conn, protocols.MsgTypeFullServerResponse, protocols.EventType_ConnectionStarted)
			if err != nil {
				return err
			}
			sessionID := uuid.New().String()
			if taskID == "" {
				taskID = sessionID
			}
			payload, err := json.Marshal(&reqParams)
			if err != nil {
				return fmt.Errorf("Failed to parse payload, %w", err)
			}
			// Start session [event=100] -----------> server
			if err := protocols.StartSession(conn, payload, sessionID); err != nil {
				return fmt.Errorf("Failed to start session, %w", err)
			}
			// Session started [event=150] <---------- server
			_, err = protocols.WaitForEvent(conn, protocols.MsgTypeFullServerResponse, protocols.EventType_SessionStarted)
			if err != nil {
				return err
			}
			// Finish session [event=102] -----------> server
			if err := protocols.FinishSession(conn, sessionID); err != nil {
				return err
			}
			voice := ""
			currentRound := 0
			for {
				var msg *protocols.Message
				// 接收响应内容
				if msg, err = protocols.ReceiveMessage(conn); err != nil {
					return fmt.Errorf("Failed to receive message, %w", err)
				}
				switch msg.MsgType {
				// 音频数据块
				case protocols.MsgTypeAudioOnlyServer:
					glog.Infof("收到音频数据块 | 大小: %d 字节", len(msg.Payload))
					// 错误信息
				case protocols.MsgTypeError:
					return fmt.Errorf("Received error message, %s", string(payload))
					// 其他消息类型
				case protocols.MsgTypeFullServerResponse:
					switch msg.EventType {
					case protocols.EventType_PodcastRoundStart:
						// 播客round开始
						var data map[string]interface{}
						if err := json.Unmarshal(msg.Payload, &data); err != nil {
							return fmt.Errorf("Failed to parse podcast round start payload, %w", err)
						}
						voice, _ = data["speaker"].(string)
						text, _ := data["text"].(string)
						if flagOnlyNlpText {
							podcastTexts = append(podcastTexts, map[string]interface{}{
								"speaker": voice,
								"text":    text,
							})
						}
						currentRound = int(data["round_id"].(float64))
						if int(currentRound) == -1 {
							voice = "head_music"
						}
						if int(currentRound) == 9999 {
							voice = "tail_music"
						}

						isPodcastRoundEnd = false
						glog.Infof("新的一轮开始: %v", data)

						// 调用进度回调，更新时间戳
						if progressCallback != nil {
							progressCallback()
						}
					case protocols.EventType_PodcastRoundEnd:
						var audioDuration AudioRoundDuration
						if err := json.Unmarshal(msg.Payload, &audioDuration); err != nil {
							return fmt.Errorf("Failed to parse meta info, %w", err)
						}
						durations = durations.Add(decimal.NewFromFloat(audioDuration.AudioDuration))
					case protocols.EventType_PodcastEnd:
						if err := json.Unmarshal(msg.Payload, &metaInfo); err != nil {
							return fmt.Errorf("Failed to parse meta info, %w", err)
						}
						result.Meta = metaInfo
					case protocols.EventType_UsageResponse:
						if err := json.Unmarshal(msg.Payload, &usage); err != nil {
							return fmt.Errorf("Failed to parse usage, %w", err)
						}
						result.Usage = usage
					}
				}
				// 会话结束
				if msg.EventType == protocols.EventType_SessionFinished {
					isPodcastRoundEnd = true
					break
				}
			}
			return nil
		}()
		// 播客结束, 保存最终音频文件
		if isPodcastRoundEnd {
			break
		} else {
			glog.Infof("播客未结束，进入第 %d 轮", lastRoundID)
			retryNum--
			time.Sleep(1 * time.Second)
		}
	}

	result.AudioDuration = durations.InexactFloat64()
	return &result, err
}

type Usage struct {
	Usage struct {
		InputTextTokens   int64 `json:"input_text_tokens"`
		OutputAudioTokens int64 `json:"output_audio_tokens"`
	} `json:"usage"`
}

type MetaInfo struct {
	MetaInfo struct {
		AudioUrl string `json:"audio_url"`
	} `json:"meta_info"`
}

type Result struct {
	Meta          MetaInfo `json:"meta"`
	Usage         Usage    `json:"usage"`
	AudioDuration float64  `json:"audio_duration"`
	RequestID     string   `json:"request_id"`
}

type AudioRoundDuration struct {
	AudioDuration float64 `json:"audio_duration"`
}
