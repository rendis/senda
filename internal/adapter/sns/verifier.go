package sns

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// snsCertHostRe matches only sns.{region}.amazonaws.com hosts.
// The region pattern allows standard AWS region identifiers (e.g., us-east-1, eu-west-1, ap-southeast-1).
var snsCertHostRe = regexp.MustCompile(`^sns\.[a-z]{2}(-[a-z]+-\d+)\.amazonaws\.com$`)

var (
	ErrInvalidSignatureVersion = errors.New("sns: unsupported signature version")
	ErrInvalidCertURL          = errors.New("sns: signing cert URL is not a valid amazonaws.com endpoint")
	ErrCertFetch               = errors.New("sns: failed to fetch signing certificate")
	ErrCertParse               = errors.New("sns: failed to parse signing certificate")
	ErrSignatureInvalid        = errors.New("sns: signature verification failed")
	ErrMalformedMessage        = errors.New("sns: malformed SNS message")
)

// snsMessageFields is the minimal set of fields needed for signature verification.
type snsMessageFields struct {
	Type             string `json:"Type"`
	MessageId        string `json:"MessageId"`
	TopicArn         string `json:"TopicArn"`
	Message          string `json:"Message"`
	Timestamp        string `json:"Timestamp"`
	Subject          string `json:"Subject,omitempty"`
	Token            string `json:"Token,omitempty"`
	SubscribeURL     string `json:"SubscribeURL,omitempty"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertURL"`
}

const (
	// certCacheMaxSize is the maximum number of entries in the certificate cache.
	certCacheMaxSize = 100
	// certCacheTTL is the time-to-live for cached certificates.
	certCacheTTL = 1 * time.Hour
)

// certCacheEntry wraps a certificate with its cache timestamp.
type certCacheEntry struct {
	cert      *x509.Certificate
	cachedAt  time.Time
}

// Verifier verifies the authenticity of SNS messages using certificate-based signature verification.
// NOTE: This verifier handles signature verification only. SubscriptionConfirmation URL validation
// and fetching is handled by the webhook handler layer (provider_webhook.go), which validates
// the SubscribeURL before passing it to the SubscriptionConfirmer.
type Verifier struct {
	httpClient *http.Client
	certCache  sync.Map // url -> *certCacheEntry
	cacheSize  int64    // atomic-ish counter (protected by certCache operations)
	mu         sync.Mutex // protects cache eviction
}

// NewVerifier creates a new SNS message verifier.
func NewVerifier(client *http.Client) *Verifier {
	if client == nil {
		client = http.DefaultClient
	}
	return &Verifier{httpClient: client}
}

// Verify checks the SNS message signature against the signing certificate.
func (v *Verifier) Verify(message []byte) error {
	var msg snsMessageFields
	if err := json.Unmarshal(message, &msg); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformedMessage, err)
	}

	// Determine the hash algorithm from the signature version.
	// Version "1" uses SHA1; version "2" uses SHA256 (recommended since 2022).
	var hashAlg crypto.Hash
	switch msg.SignatureVersion {
	case "1":
		hashAlg = crypto.SHA1
	case "2":
		hashAlg = crypto.SHA256
	default:
		return fmt.Errorf("%w: got %q", ErrInvalidSignatureVersion, msg.SignatureVersion)
	}

	// Validate the cert URL domain.
	if err := validateCertURL(msg.SigningCertURL); err != nil {
		return err
	}

	// Fetch (or get from cache) the signing certificate.
	cert, err := v.fetchCert(msg.SigningCertURL)
	if err != nil {
		return err
	}

	// Build the string-to-sign based on message type.
	stringToSign := buildStringToSign(&msg)

	// Decode the base64 signature.
	sig, err := base64.StdEncoding.DecodeString(msg.Signature)
	if err != nil {
		return fmt.Errorf("%w: invalid base64 signature", ErrSignatureInvalid)
	}

	// Verify using RSA + PKCS1v15 with the selected hash algorithm.
	rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("%w: certificate does not contain an RSA public key", ErrCertParse)
	}

	hashed := hashAlg.New()
	hashed.Write([]byte(stringToSign))
	if err := rsa.VerifyPKCS1v15(rsaKey, hashAlg, hashed.Sum(nil), sig); err != nil {
		return ErrSignatureInvalid
	}

	return nil
}

// validateCertURL ensures the signing certificate URL points to an AWS SNS endpoint.
// Only sns.{region}.amazonaws.com hosts are allowed (not arbitrary *.amazonaws.com).
func validateCertURL(certURL string) error {
	parsed, err := url.Parse(certURL)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCertURL, err)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be https", ErrInvalidCertURL)
	}

	// Must be exactly sns.{region}.amazonaws.com — not any amazonaws.com subdomain.
	if !snsCertHostRe.MatchString(parsed.Host) {
		return fmt.Errorf("%w: host %q is not a valid SNS endpoint (expected sns.<region>.amazonaws.com)", ErrInvalidCertURL, parsed.Host)
	}

	return nil
}

// fetchCert downloads and caches the signing certificate.
// Cached entries expire after certCacheTTL. The cache is cleared when it exceeds certCacheMaxSize.
func (v *Verifier) fetchCert(certURL string) (*x509.Certificate, error) {
	// Check cache first — verify TTL.
	if cached, ok := v.certCache.Load(certURL); ok {
		entry := cached.(*certCacheEntry)
		if time.Since(entry.cachedAt) < certCacheTTL {
			return entry.cert, nil
		}
		// Expired — remove and re-fetch.
		v.certCache.Delete(certURL)
		v.decrementCacheSize()
	}

	resp, err := v.httpClient.Get(certURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCertFetch, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrCertFetch, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCertFetch, err)
	}

	block, _ := pem.Decode(body)
	if block == nil {
		return nil, fmt.Errorf("%w: no PEM block found", ErrCertParse)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCertParse, err)
	}

	// Evict entire cache if it exceeds the size limit.
	v.evictIfNeeded()

	v.certCache.Store(certURL, &certCacheEntry{
		cert:     cert,
		cachedAt: time.Now(),
	})
	v.incrementCacheSize()

	return cert, nil
}

// evictIfNeeded clears the cache if it exceeds the maximum size.
func (v *Verifier) evictIfNeeded() {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.cacheSize >= certCacheMaxSize {
		v.certCache.Range(func(key, _ any) bool {
			v.certCache.Delete(key)
			return true
		})
		v.cacheSize = 0
	}
}

func (v *Verifier) incrementCacheSize() {
	v.mu.Lock()
	v.cacheSize++
	v.mu.Unlock()
}

func (v *Verifier) decrementCacheSize() {
	v.mu.Lock()
	if v.cacheSize > 0 {
		v.cacheSize--
	}
	v.mu.Unlock()
}

// buildStringToSign constructs the canonical string-to-sign for an SNS message.
// See: https://docs.aws.amazon.com/sns/latest/dg/sns-verify-signature-of-message.html
func buildStringToSign(msg *snsMessageFields) string {
	var b strings.Builder

	switch msg.Type {
	case "Notification":
		b.WriteString("Message\n")
		b.WriteString(msg.Message)
		b.WriteString("\n")
		b.WriteString("MessageId\n")
		b.WriteString(msg.MessageId)
		b.WriteString("\n")
		if msg.Subject != "" {
			b.WriteString("Subject\n")
			b.WriteString(msg.Subject)
			b.WriteString("\n")
		}
		b.WriteString("Timestamp\n")
		b.WriteString(msg.Timestamp)
		b.WriteString("\n")
		b.WriteString("TopicArn\n")
		b.WriteString(msg.TopicArn)
		b.WriteString("\n")
		b.WriteString("Type\n")
		b.WriteString(msg.Type)
		b.WriteString("\n")

	case "SubscriptionConfirmation", "UnsubscribeConfirmation":
		b.WriteString("Message\n")
		b.WriteString(msg.Message)
		b.WriteString("\n")
		b.WriteString("MessageId\n")
		b.WriteString(msg.MessageId)
		b.WriteString("\n")
		b.WriteString("SubscribeURL\n")
		b.WriteString(msg.SubscribeURL)
		b.WriteString("\n")
		b.WriteString("Timestamp\n")
		b.WriteString(msg.Timestamp)
		b.WriteString("\n")
		b.WriteString("Token\n")
		b.WriteString(msg.Token)
		b.WriteString("\n")
		b.WriteString("TopicArn\n")
		b.WriteString(msg.TopicArn)
		b.WriteString("\n")
		b.WriteString("Type\n")
		b.WriteString(msg.Type)
		b.WriteString("\n")
	}

	return b.String()
}
