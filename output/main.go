package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/K3das/riverfish/proto/go/pipeline"
	"github.com/bwmarrin/discordgo"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

const PROXY = "socks5://tor:9150"

const (
	WEBHOOK_ID    = "[REDACTED]"
	WEBHOOK_TOKEN = "[REDACTED]"
)

type torCheckResponse struct {
	IsTor bool   `json:"IsTor"`
	IP    string `json:"IP"`
}

var s *discordgo.Session

func isTOR(httpClient *http.Client) bool {
	res, err := httpClient.Get("https://check.torproject.org/api/ip")
	if err != nil {
		log.Fatalf("couldn't fetch ip: %v", err)
	}
	defer res.Body.Close()

	var checkResponse torCheckResponse
	err = json.NewDecoder(res.Body).Decode(&checkResponse)
	if err != nil {
		log.Fatalf("couldn't decode tor check response: %v", err)
	}

	return checkResponse.IsTor
}

func getClient() *http.Client {
	url_i := url.URL{}
	url_proxy, _ := url_i.Parse(PROXY)
	transport := http.Transport{}
	transport.Proxy = http.ProxyURL(url_proxy)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	httpClient := &http.Client{Transport: &transport}
	return httpClient
}

func main() {
	httpClient := getClient()

	if !isTOR(httpClient) {
		log.Fatalln("tor not working, quitting")
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ProxyServer(PROXY),
		chromedp.IgnoreCertErrors,
		chromedp.DisableGPU,
		chromedp.Flag("incognito", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36"),
	)
	actx, acancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer acancel()
	cctx, cancel := chromedp.NewContext(actx)
	defer cancel()

	var err error
	err = chromedp.Run(cctx, chromedp.ActionFunc(func(_ context.Context) error {
		return nil
	}))
	if err != nil {
		log.Fatalln(err)
	}

	s, err = discordgo.New("")
	if err != nil {
		log.Fatalf("bot fucked: %v", err)
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{os.Getenv("PANDA")},
		Topic:     os.Getenv("OUTPUT_TOPIC"),
		GroupID:   "screenshots",
		Partition: 0,
	})

	logged := make(map[string]struct{})

	log.Println("haiii")

	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			break
		}
		var cert = pipeline.ScoredCertificateLog{}
		if err := proto.Unmarshal(m.Value, &cert); err != nil {
			log.Fatalln(err)
		}

		if _, ok := logged[cert.Subject]; ok {
			continue
		}
		if len(logged) > 1000 {
			log.Println("clearing massive history")
			logged = make(map[string]struct{})
		}
		logged[cert.Subject] = struct{}{}

		sendWebhook(&cert, cctx)
	}
}

func sendWebhook(cert *pipeline.ScoredCertificateLog, xctx context.Context) {
	cctx, xCancel := context.WithTimeout(xctx, time.Second*30)
	defer xCancel()
	log.Printf("alerting for %s", cert.Subject)
	params := discordgo.WebhookParams{}
	embed := discordgo.MessageEmbed{
		Description: fmt.Sprintf("`%s`", cert.Subject),
		Color:       16738919,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "yarn.network",
		},
	}

	var buf []byte
	if err := chromedp.Run(cctx, chromedp.Tasks{
		chromedp.EmulateViewport(1280, 960),
		chromedp.Navigate(fmt.Sprintf("https://%s/", cert.Subject)),
		chromedp.CaptureScreenshot(&buf),
	}); err == nil {
		filename := fmt.Sprintf("%s.png", uuid.New().String())
		embed.Image = &discordgo.MessageEmbedImage{
			URL: fmt.Sprintf("attachment://%s", filename),
		}

		reader := bytes.NewReader(buf)
		params.Files = []*discordgo.File{
			{
				Name:   filename,
				Reader: reader,
			},
		}
	}

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "`score` (higher is sussier)",
			Value:  fmt.Sprintf("`%f`", cert.TotalScore),
			Inline: true,
		},
		{
			Name:   "`alg_score` (higher is sussier)",
			Value:  fmt.Sprintf("`%f`", cert.AlgScore),
			Inline: true,
		},
		{
			Name:   "`ml_score` (lower is sussier)",
			Value:  fmt.Sprintf("`%f`", cert.MlScore),
			Inline: false,
		},
	}
	embed.Timestamp = time.Now().UTC().Format(time.RFC3339)
	params.Embeds = []*discordgo.MessageEmbed{&embed}

	s.WebhookExecute(WEBHOOK_ID, WEBHOOK_TOKEN, false, &params)
	log.Printf("done alerting for %s", cert.Subject)
}
