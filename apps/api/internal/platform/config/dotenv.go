package config

import (
    "bufio"
    "os"
    "path/filepath"
    "strings"
)

func LoadDotEnv() {
    dir, err := os.Getwd()
    if err != nil {
        return
    }
    for {
        candidate := filepath.Join(dir, ".env")
        if _, err := os.Stat(candidate); err == nil {
            applyEnvFile(candidate)
            return
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            return
        }
        dir = parent
    }
}

func applyEnvFile(path string) {
    f, err := os.Open(path)
    if err != nil {
        return
    }
    defer f.Close()

    sc := bufio.NewScanner(f)
    for sc.Scan() {
        line := strings.TrimSpace(sc.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        eq := strings.IndexByte(line, '=')
        if eq <= 0 {
            continue
        }
        key := strings.TrimSpace(line[:eq])
        val := strings.TrimSpace(line[eq+1:])
        if len(val) >= 2 {
            if (val[0] == '"' && val[len(val)-1] == '"') ||
                (val[0] == '\'' && val[len(val)-1] == '\'') {
                val = val[1 : len(val)-1]
            }
        }
        if _, exists := os.LookupEnv(key); !exists {
            _ = os.Setenv(key, val)
        }
    }
}