package types

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yanyiwu/gojieba"
)

// gojieba v1.4.7's dictionaries. Its default paths point into the build machine's module cache,
// so a shipped Lite or desktop binary would not find them.
//
//go:embed jiebadict/*.utf8
var jiebaDicts embed.FS

// The order gojieba.NewJieba takes its paths in.
var jiebaDictFiles = [...]string{"jieba.dict.utf8", "hmm_model.utf8", "user.dict.utf8", "idf.utf8", "stop_words.utf8"}

// Jieba is a global instance of Chinese text segmentation tool
var Jieba = newJieba()

func newJieba() *gojieba.Jieba {
	dir := os.Getenv("JIEBA_DICT_DIR")
	if dir == "" {
		// Docker images copy the module cache, and dev builds run where they were built.
		if _, err := os.Stat(gojieba.DICT_PATH); err == nil {
			return gojieba.NewJieba()
		}
		var err error
		if dir, err = writeJiebaDicts(); err != nil {
			panic(fmt.Sprintf("jieba dictionaries: %v", err))
		}
	}
	paths := make([]string, len(jiebaDictFiles))
	for i, f := range jiebaDictFiles {
		paths[i] = filepath.Join(dir, f)
	}
	return gojieba.NewJieba(paths...)
}

// writeJiebaDicts copies the embedded dictionaries into a directory named by their content hash,
// under the user cache dir or else the temp dir, and returns it. Files already there are kept.
func writeJiebaDicts() (string, error) {
	data := make([][]byte, len(jiebaDictFiles))
	h := sha256.New()
	for i, f := range jiebaDictFiles {
		b, err := jiebaDicts.ReadFile("jiebadict/" + f)
		if err != nil {
			return "", err
		}
		data[i] = b
		h.Write(b)
	}
	name := "jieba-" + hex.EncodeToString(h.Sum(nil))[:12]

	var errs []error
	for _, base := range jiebaCacheBases() {
		dir := filepath.Join(base, "ai-kb", name)
		err := writeJiebaDir(dir, data)
		if err == nil {
			return dir, nil
		}
		errs = append(errs, err)
	}
	return "", errors.Join(errs...)
}

func jiebaCacheBases() []string {
	if d, err := os.UserCacheDir(); err == nil {
		return []string{d, os.TempDir()}
	}
	return []string{os.TempDir()}
}

func writeJiebaDir(dir string, data [][]byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, f := range jiebaDictFiles {
		dst := filepath.Join(dir, f)
		if st, err := os.Stat(dst); err == nil && st.Size() == int64(len(data[i])) {
			continue
		}
		// Write then rename, so a concurrent start never reads a half-written file.
		tmp, err := os.CreateTemp(dir, f+".*")
		if err != nil {
			return err
		}
		_, werr := tmp.Write(data[i])
		cerr := tmp.Close()
		if err := errors.Join(werr, cerr); err != nil {
			_ = os.Remove(tmp.Name())
			return err
		}
		if err := os.Rename(tmp.Name(), dst); err != nil {
			_ = os.Remove(tmp.Name())
			return err
		}
	}
	return nil
}
