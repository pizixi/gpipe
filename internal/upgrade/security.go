package upgrade

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pizixi/gpipe/internal/pb"
)

const ProtocolVersion uint32 = 1

var semverPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

func SignOffer(key string, offer *pb.UpgradeOffer) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(offerPayload(offer)))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyOffer(key string, offer *pb.UpgradeOffer) bool {
	provided, err := hex.DecodeString(strings.TrimSpace(offer.Signature))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(offerPayload(offer)))
	return hmac.Equal(provided, mac.Sum(nil))
}

func offerPayload(offer *pb.UpgradeOffer) string {
	return fmt.Sprintf("gpipe-upgrade-v1\n%s\n%s\n%s\n%d\n%s\n%d",
		offer.TaskID, offer.Version, offer.Platform, offer.Size,
		strings.ToLower(offer.SHA256), offer.ChunkSize)
}

func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	// Hex digests in manifests, offers, and persisted update state are
	// canonicalized to lowercase. Readers still accept either case where data
	// may come from an older or external producer.
	return hex.EncodeToString(sum[:])
}

// IsValidVersion reports whether value is accepted by the upgrade protocol's
// semantic-version parser.
func IsValidVersion(value string) bool {
	_, ok := parseVersion(value)
	return ok
}

// CompareVersions compares semantic versions. Build metadata is ignored. The
// second result is false when either value is not a valid semantic version.
func CompareVersions(left, right string) (int, bool) {
	a, ok := parseVersion(left)
	if !ok {
		return 0, false
	}
	b, ok := parseVersion(right)
	if !ok {
		return 0, false
	}
	for i := 0; i < 3; i++ {
		if a.numbers[i] < b.numbers[i] {
			return -1, true
		}
		if a.numbers[i] > b.numbers[i] {
			return 1, true
		}
	}
	if a.pre == b.pre {
		return 0, true
	}
	if a.pre == "" {
		return 1, true
	}
	if b.pre == "" {
		return -1, true
	}
	ap, bp := strings.Split(a.pre, "."), strings.Split(b.pre, ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		if ap[i] == bp[i] {
			continue
		}
		an, ae := strconv.ParseUint(ap[i], 10, 64)
		bn, be := strconv.ParseUint(bp[i], 10, 64)
		switch {
		case ae == nil && be == nil:
			if an < bn {
				return -1, true
			}
			return 1, true
		case ae == nil:
			return -1, true
		case be == nil:
			return 1, true
		default:
			if ap[i] < bp[i] {
				return -1, true
			}
			return 1, true
		}
	}
	if len(ap) < len(bp) {
		return -1, true
	}
	return 1, true
}

type parsedVersion struct {
	numbers [3]uint64
	pre     string
}

func parseVersion(value string) (parsedVersion, bool) {
	match := semverPattern.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return parsedVersion{}, false
	}
	var out parsedVersion
	for i := 0; i < 3; i++ {
		value, err := strconv.ParseUint(match[i+1], 10, 64)
		if err != nil {
			return parsedVersion{}, false
		}
		out.numbers[i] = value
	}
	out.pre = match[4]
	return out, true
}
