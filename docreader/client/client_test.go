package client

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/docreader/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
)

func TestNewClientDoesNotChangeGlobalGRPCResolver(t *testing.T) {
	previous := resolver.GetDefaultScheme()
	resolver.SetDefaultScheme("passthrough")
	t.Cleanup(func() { resolver.SetDefaultScheme(previous) })

	client, err := NewClientWithAuth("127.0.0.1:1", &AuthConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if got := resolver.GetDefaultScheme(); got != "passthrough" {
		t.Fatalf("DocReader changed global gRPC resolver to %q", got)
	}
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.Println("INFO: Initializing DocReader client tests")
}

func requireLiveDocReaderClient(t *testing.T) *Client {
	t.Helper()

	client, err := NewClient("localhost:50051")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.ListEngines(ctx, &proto.ListEnginesRequest{}); err != nil {
		t.Skipf("DocReader gRPC server not available at localhost:50051: %v", err)
	}

	return client
}

func TestReadURL(t *testing.T) {
	client := requireLiveDocReaderClient(t)
	client.SetDebug(true)

	startTime := time.Now()
	resp, err := client.Read(
		context.Background(),
		&proto.ReadRequest{
			Url:   "https://example.com",
			Title: "test",
		},
	)
	log.Printf("INFO: Read(URL) completed in %v", time.Since(startTime))

	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("Read returned error: %s", resp.Error)
	}
	if resp.MarkdownContent == "" {
		t.Error("Expected non-empty markdown content")
	}
	log.Printf("INFO: content_len=%d, images=%d", len(resp.MarkdownContent), len(resp.ImageRefs))
}

func TestConvertSpreadsheet(t *testing.T) {
	client := requireLiveDocReaderClient(t)

	xlsb, err := os.ReadFile("../tests/fixtures/pandas-test1.xlsb")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.ConvertSpreadsheet(context.Background(), &proto.ConvertSpreadsheetRequest{
		FileContent: xlsb, FileType: "xlsb",
	})
	if status.Code(err) == codes.Unimplemented {
		// validate.sh reuses a server already on the port, such as a dev stack on an older image.
		t.Skipf("the DocReader server on localhost:50051 predates ConvertSpreadsheet: %v", err)
	}
	if err != nil {
		t.Fatalf("ConvertSpreadsheet: %v", err)
	}
	book, err := zip.NewReader(bytes.NewReader(resp.XlsxContent), int64(len(resp.XlsxContent)))
	if err != nil {
		t.Fatalf("response is not an .xlsx: %v", err)
	}
	f, err := book.Open("xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	workbook, _ := io.ReadAll(f)
	for _, sheet := range []string{`name="Sheet1"`, `name="Sheet2"`, `name="Sheet3"`, `name="Sheet4"`} {
		if !bytes.Contains(workbook, []byte(sheet)) {
			t.Errorf("xl/workbook.xml lacks %s", sheet)
		}
	}

	_, err = client.ConvertSpreadsheet(context.Background(), &proto.ConvertSpreadsheetRequest{
		FileContent: []byte("not a workbook"), FileType: "xls",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("garbage input: err = %v, want InvalidArgument", err)
	}
}

func TestReadFile(t *testing.T) {
	client := requireLiveDocReaderClient(t)
	client.SetDebug(true)

	fileContent, err := os.ReadFile("../testdata/test.md")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	startTime := time.Now()
	resp, err := client.Read(
		context.Background(),
		&proto.ReadRequest{
			FileContent: fileContent,
			FileName:    "test.md",
			FileType:    "md",
		},
	)
	log.Printf("INFO: Read(file) completed in %v", time.Since(startTime))

	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("Read returned error: %s", resp.Error)
	}
	if resp.MarkdownContent == "" {
		t.Error("Expected non-empty markdown content")
	}

	imageRefs := GetImageRefsFromResponse(resp)
	log.Printf("INFO: content_len=%d, images=%d", len(resp.MarkdownContent), len(imageRefs))
}
