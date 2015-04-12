package main

import (
	"fmt"
	// "github.com/kamichidu/go-cache"
	// "golang.org/x/net/html"
	"flag"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

const (
	// application name
	AppName = "ref-msdn (dev)"
	// application version
	AppVersion = "0.0.0"
	// a root url for msdn library
	MsdnRootUrl = "https://msdn.microsoft.com/library"
)

// usage: msdn-viewer [-l] [-C] [-c=/path/to/dir/] {query}...
// query are:
//
// command-line arguments
var (
	show_list       = flag.Bool("l", false, "Print list of content.")
	clear_cache     = flag.Bool("C", false, "Clear cache data.")
	cache_directory = flag.String("c", os.ExpandEnv("$TEMP"), "The cache directory path.")
	user_agent      = flag.String("U", fmt.Sprintf("%s - %s", AppName, AppVersion), "The user agent.")
	debug           = flag.Bool("debug", false, "DO NOT USE THIS.")
	debug_filter    = flag.Bool("debug-filter", false, "DO NOT USE THIS.")
)

type Item struct {
	Tag  string
	Link string
}

func url2filename(url string) string {
	const no_limit = -1
	filename := url
	filename = strings.Replace(filename, "/", "_", no_limit)
	filename = strings.Replace(filename, ">", "_", no_limit)
	filename = strings.Replace(filename, "<", "_", no_limit)
	filename = strings.Replace(filename, "?", "_", no_limit)
	filename = strings.Replace(filename, ":", "_", no_limit)
	filename = strings.Replace(filename, "\"", "_", no_limit)
	filename = strings.Replace(filename, "\\", "_", no_limit)
	filename = strings.Replace(filename, "*", "_", no_limit)
	filename = strings.Replace(filename, "|", "_", no_limit)
	filename = strings.Replace(filename, ";", "_", no_limit)
	return *cache_directory + "/" + filename
}

func hasCache(url string) bool {
	if *clear_cache {
		return false
	}
	_, err := os.Stat(url2filename(url))
	if err != nil {
		return false
	}
	return true
}

func downloadPage(url string) (*goquery.Document, error) {
	// when has cache
	if hasCache(url) {
		file, err := os.Open(url2filename(url))
		if err != nil {
			return nil, err
		}

		doc, err := goquery.NewDocumentFromReader(file)
		if err != nil {
			return nil, err
		}
		return doc, nil
	} else {
		request, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		request.Header.Set("User-Agent", *user_agent)

		client := &http.Client{}
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		doc, err := goquery.NewDocumentFromReader(response.Body)
		if err != nil {
			return nil, err
		}

		{
			s, err := doc.Html()
			if err == nil {
				ioutil.WriteFile(url2filename(url), []byte(s), os.ModePerm)
			}
		}

		return doc, nil
	}
}

func filter(unfiltered []*Item, tag string) []*Item {
	if tag == "" {
		return unfiltered
	}

	var filtered []*Item
	for _, item := range unfiltered {
		if *debug_filter {
			fmt.Printf("------ matching (%s == %s) => %v\n", item.Tag, tag, item.Tag == tag)
		}
		if item.Tag == tag {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func parseCatalog1() []*Item {
	doc, err := downloadPage(MsdnRootUrl)
	if err != nil {
		panic(err)
	}

	items := []*Item{}
	doc.Find("div.catalog>h2").Each(func(_ int, s *goquery.Selection) {
		items = append(items, &Item{
			Tag: strings.TrimSpace(s.Text()),
		})
	})
	return items
}

func parseCatalog2(catalog []*Item) []*Item {
	doc, err := downloadPage(MsdnRootUrl)
	if err != nil {
		panic(err)
	}

	var items []*Item
	doc.Find("div.catalog>h2").Each(func(_ int, s *goquery.Selection) {
		if len(filter(catalog, strings.TrimSpace(s.Text()))) <= 0 {
			return
		}

		s.Next().Children().Each(func(_ int, s *goquery.Selection) {
			link, _ := s.Find("a").Attr("href")
			items = append(items, &Item{
				Tag:  strings.TrimSpace(s.Text()),
				Link: link,
			})
		})
	})
	return items
}

func parseNavpage(url string) []*Item {
	doc, err := downloadPage(url)
	if err != nil {
		panic(err)
	}

	var items []*Item
	doc.Find("div.topic div.sectionblock dl.authored > dt").Children().Each(func(_ int, s *goquery.Selection) {
		link, _ := s.Find("a").Attr("href")
		items = append(items, &Item{
			Tag:  strings.TrimSpace(s.Text()),
			Link: link,
		})
	})
	return items
}

func main() {
	flag.Parse()

	if flag.NArg() <= 0 {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	queries := append(flag.Args(), "")
	var items []*Item
	for i, query := range queries {
		if i == 0 {
			items = parseCatalog1()
			if *debug {
				fmt.Printf("-- Query0=%s\n", query)
				for _, item := range items {
					fmt.Printf("---- item=%v\n", item)
				}
			}
			items = filter(items, query)
			if *debug {
				fmt.Println("-- After filter")
				for _, item := range items {
					fmt.Printf("---- item=%v\n", item)
				}
			}
		} else if i == 1 {
			items = parseCatalog2(items)
			if *debug {
				fmt.Printf("-- Query1=%s\n", query)
				for _, item := range items {
					fmt.Printf("---- item=%v\n", item)
				}
			}
			items = filter(items, query)
			if *debug {
				fmt.Println("-- After filter")
				for _, item := range items {
					fmt.Printf("---- item=%v\n", item)
				}
			}
		} else {
			if *debug {
				fmt.Printf("-- Query=%s\n", query)
				for _, item := range items {
					fmt.Printf("---- item=%v\n", item)
				}
			}
			var buf_items []*Item
			for _, item := range items {
				for _, e := range parseNavpage(item.Link) {
					buf_items = append(buf_items, e)
				}
			}
			items = filter(buf_items, query)
			if *debug {
				fmt.Println("-- After filter")
				for _, item := range items {
					fmt.Printf("---- item=%v\n", item)
				}
			}
		}
	}

	for _, item := range items {
		fmt.Printf("tag=%s, link=%s\n", item.Tag, item.Link)
	}
}
