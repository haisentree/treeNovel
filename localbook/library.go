package localbook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 可解析的电子书扩展名（MOBI7 家族；.azw3 是 KF8 结构，另行报错提示）。
var SupportedExt = []string{".azw", ".mobi", ".prc"}

// Library book/ 目录的内存缓存层：Web 请求直接读原文件，
// 解析结果按「文件名 + 修改时间 + 大小」缓存，文件被替换后自动失效。
type Library struct {
	Dir string

	mu     sync.Mutex
	loaded map[string]*cacheEntry
}

type cacheEntry struct {
	modTime time.Time
	size    int64
	book    *Book
}

// NewLibrary 创建指向 dir 的书库（目录可以不存在，List 时再报错）。
func NewLibrary(dir string) *Library {
	return &Library{Dir: dir, loaded: make(map[string]*cacheEntry)}
}

// Item 书库列表里的一本书。
type Item struct {
	Name   string // 文件名（URL 里用）
	Title  string
	Author string
	Chars  int    // 全书正文字数
	Notes  string // 不可读原因（扩展名支持但解析失败时）
	OK     bool
}

// List 扫描目录，返回全部电子书（解析失败的也会列出并带原因）。
func (l *Library) List() ([]Item, error) {
	entries, err := os.ReadDir(l.Dir)
	if err != nil {
		return nil, err
	}
	var items []Item
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !contains(SupportedExt, ext) {
			continue
		}
		item := Item{Name: name}
		book, err := l.Get(name)
		switch {
		case err == nil:
			item.Title, item.Author, item.Chars, item.OK = book.Meta.Title, book.Meta.Author, book.totalChars(), true
			if item.Title == "" {
				item.Title = baseTitle(name)
			}
		default:
			item.Title, item.Notes = baseTitle(name), err.Error()
		}
		items = append(items, item)
	}
	return items, nil
}

// Get 取一本书（带缓存），name 只取基名防止路径穿越。
func (l *Library) Get(name string) (*Book, error) {
	name = filepath.Base(name)
	if !contains(SupportedExt, strings.ToLower(filepath.Ext(name))) {
		return nil, fmt.Errorf("不支持的文件类型: %s", name)
	}
	path := filepath.Join(l.Dir, name)

	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	l.mu.Lock()
	if c, ok := l.loaded[name]; ok && c.modTime.Equal(st.ModTime()) && c.size == st.Size() {
		l.mu.Unlock()
		return c.book, nil
	}
	l.mu.Unlock()

	// 解析放在锁外：单文件书解析约几十毫秒，重复解析可接受，不阻塞并发读
	book, err := ParseFile(path)
	if err != nil {
		return nil, err
	}

	l.mu.Lock()
	l.loaded[name] = &cacheEntry{modTime: st.ModTime(), size: st.Size(), book: book}
	l.mu.Unlock()
	return book, nil
}

// Path 文件完整路径（下载用），同样只允许基名。
func (l *Library) Path(name string) (string, error) {
	name = filepath.Base(name)
	if !contains(SupportedExt, strings.ToLower(filepath.Ext(name))) {
		return "", fmt.Errorf("不支持的文件类型: %s", name)
	}
	p := filepath.Join(l.Dir, name)
	if _, err := os.Stat(p); err != nil {
		return "", err
	}
	return p, nil
}

func (b *Book) totalChars() int {
	n := 0
	for _, s := range b.Sections {
		n += s.Chars
	}
	return n
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// baseTitle 从文件名提取书名：去掉扩展名与「 - 作者」尾巴（元数据缺失时兜底）。
func baseTitle(name string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if i := strings.LastIndex(name, " - "); i > 0 {
		name = name[:i]
	}
	return stripBookBrackets(strings.TrimSpace(name))
}
