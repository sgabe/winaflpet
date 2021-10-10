package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/rs/xid"
	"github.com/spf13/cast"
	"golang.org/x/crypto/argon2"
)

type APIResponse struct {
	GUID xid.ID `json:"guid"`
	PID  int    `json:"pid"`
	Msg  string `json:"msg"`
	Err  string `json:"error"`
}

type passwordConfig struct {
	time    uint32
	memory  uint32
	threads uint8
	keyLen  uint32
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func fileEmpty(filename string) bool {
	if fileExists(filename) {
		if info, _ := os.Stat(filename); info.Size() > 0 {
			return false
		}
	}
	return true
}

func formatDuration(timestamp int) string {
	if timestamp == 0 {
		return "N/A"
	}

	t := time.Unix(int64(timestamp-11644473600), 0)
	e := time.Since(t)

	d := int(e.Hours()) / 24
	h := int(e.Hours()) % 24
	m := int(e.Minutes()) % 60
	s := int(e.Seconds()) % 60

	return fmt.Sprintf("%d days, %d hrs, %d min, %d sec\n", d, h, m, s)
}

func generateSecretKey() []byte {
	b := make([]byte, 32)
	rand.Read(b)
	return b
}

func getVersion() string {
	return fmt.Sprintf("%s (rev %s)", BuildVer, BuildRev)
}

func seq(args ...interface{}) ([]int, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, errors.New("invalid number of arguments to Seq")
	}

	intArgs := cast.ToIntSlice(args)
	if len(intArgs) < 1 || len(intArgs) > 3 {
		return nil, errors.New("invalid arguments to Seq")
	}

	var inc = 1
	var last int
	var first = intArgs[0]

	if len(intArgs) == 1 {
		last = first
		if last == 0 {
			return []int{}, nil
		} else if last > 0 {
			first = 1
		} else {
			first = -1
			inc = -1
		}
	} else if len(intArgs) == 2 {
		last = intArgs[1]
		if last < first {
			inc = -1
		}
	} else {
		inc = intArgs[1]
		last = intArgs[2]
		if inc == 0 {
			return nil, errors.New("'increment' must not be 0")
		}
		if first < last && inc < 0 {
			return nil, errors.New("'increment' must be > 0")
		}
		if first > last && inc > 0 {
			return nil, errors.New("'increment' must be < 0")
		}
	}

	// sanity check
	if last < -100000 {
		return nil, errors.New("size of result exceeds limit")
	}
	size := ((last - first) / inc) + 1

	// sanity check
	if size <= 0 || size > 2000 {
		return nil, errors.New("size of result exceeds limit")
	}

	seq := make([]int, size)
	val := first
	for i := 0; ; i++ {
		seq[i] = val
		val += inc
		if (inc < 0 && val < last) || (inc > 0 && val > last) {
			break
		}
	}

	return seq, nil
}

func formatNumber(num interface{}) string {
	x, isFloat := num.(float64)
	if !isFloat {
		x = float64(num.(int))
	}

	xNum := numberFormat(float64(numberRoundInt(x)), 2, ".", ",")

	if math.Abs(x) < 999.5 {
		xNumStr := xNum[:len(xNum)-3]
		return string(xNumStr)
	}

	// first, remove the .00 then convert to slice
	xNumStr := xNum[:len(xNum)-3]
	xNumCleaned := strings.Replace(xNumStr, ",", " ", -1)
	xNumSlice := strings.Fields(xNumCleaned)
	count := len(xNumSlice) - 2
	unit := [4]string{"k", "m", "b", "t"}
	xPart := unit[count]

	afterDecimal := ""
	if xNumSlice[1][0] != 0 {
		afterDecimal = "." + string(xNumSlice[1][0])
	}
	final := xNumSlice[0] + afterDecimal + xPart
	return final
}

func numberFormat(number float64, decimals int, decPoint, thousandsSep string) string {
	if math.IsNaN(number) || math.IsInf(number, 0) {
		number = 0
	}

	var ret string
	var negative bool

	if number < 0 {
		number *= -1
		negative = true
	}

	d, fract := math.Modf(number)

	if decimals <= 0 {
		fract = 0
	} else {
		pow := math.Pow(10, float64(decimals))
		fract = numberRoundPrec(fract*pow, 0)
	}

	if thousandsSep == "" {
		ret = strconv.FormatFloat(d, 'f', 0, 64)
	} else if d >= 1 {
		var x float64
		for d >= 1 {
			d, x = math.Modf(d / 1000)
			x = x * 1000
			ret = strconv.FormatFloat(x, 'f', 0, 64) + ret
			if d >= 1 {
				ret = thousandsSep + ret
			}
		}
	} else {
		ret = "0"
	}

	fracts := strconv.FormatFloat(fract, 'f', 0, 64)

	// "0" pad left
	for i := len(fracts); i < decimals; i++ {
		fracts = "0" + fracts
	}

	ret += decPoint + fracts

	if negative {
		ret = "-" + ret
	}
	return ret
}

func numberRound(x float64) int {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}

	val := numberRoundPrec(x, 0)

	return int(val)
}

func numberRoundPrec(x float64, prec int) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}

	sign := 1.0
	if x < 0 {
		sign = -1
		x *= -1
	}

	var rounder float64
	pow := math.Pow(10, float64(prec))
	intermed := x * pow
	_, frac := math.Modf(intermed)

	if frac >= 0.5 {
		rounder = math.Ceil(intermed)
	} else {
		rounder = math.Floor(intermed)
	}

	return rounder / pow * sign
}

func numberRoundInt(input float64) int {
	var result float64

	if input < 0 {
		result = math.Ceil(input - 0.5)
	} else {
		result = math.Floor(input + 0.5)
	}

	i, _ := math.Modf(result)

	return int(i)
}

func generatePassword(password string) (string, error) {
	var c = &passwordConfig{
		time:    1,
		memory:  64 * 1024,
		threads: 4,
		keyLen:  32,
	}

	// Generate a salt.
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, c.time, c.memory, c.threads, c.keyLen)

	// Base64 encode the salt and hashed password.
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	format := "$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s"
	full := fmt.Sprintf(format, argon2.Version, c.memory, c.time, c.threads, b64Salt, b64Hash)
	return full, nil
}

func comparePassword(password, hash string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		return false, errors.New("corrupted password hash")
	}

	c := &passwordConfig{}
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &c.memory, &c.time, &c.threads)
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	c.keyLen = uint32(len(decodedHash))

	comparisonHash := argon2.IDKey([]byte(password), salt, c.time, c.memory, c.threads, c.keyLen)

	return (subtle.ConstantTimeCompare(decodedHash, comparisonHash) == 1), nil
}

func totalPages() int {
	count := 0
	size := 99

	items := squirrel.Select("COUNT(*)").From(TB_NAME_CRASHES)
	items.RunWith(db).QueryRow().Scan(&count)

	pages := math.Ceil(float64(count) / float64(size))

	return int(pages)
}
