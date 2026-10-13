package sandbox

import "testing"

func TestIsStandardTemplateRecognizesProviderScopedName(t *testing.T) {
	for _, name := range []string{"aikb", "team/aikb", "project-b89e/AIKB"} {
		if !isStandardTemplate(name) {
			t.Fatalf("expected %q to identify the AI KB standard template", name)
		}
	}
	if isStandardTemplate("ai-kb-custom") {
		t.Fatal("custom template must not be treated as the standard template")
	}
	if isStandardTemplate(DesktopTemplateName) {
		t.Fatal("the desktop sibling must not be classified as the CLI standard template")
	}
}

func TestIsDesktopTemplateRecognizesProviderScopedName(t *testing.T) {
	for _, name := range []string{"ai-kb-desktop", "team/ai-kb-desktop", "project-b89e/AI-KB-Desktop"} {
		if !isDesktopTemplate(name) {
			t.Fatalf("expected %q to identify the AI KB desktop template", name)
		}
	}
	if isDesktopTemplate("aikb") {
		t.Fatal("the CLI template must not be classified as desktop")
	}
	if isDesktopTemplate("ai-kb-desktop-custom") {
		t.Fatal("a similarly prefixed custom name must not be the desktop template")
	}
}

func TestClassifyAIKBTemplatePrefersNameOverImage(t *testing.T) {
	standard, desktop := classifyAIKBTemplate(DesktopTemplateName, DefaultDockerImage)
	if standard || !desktop {
		t.Fatalf(
			"named desktop template must be desktop even if the image repo matches CLI, got standard=%v desktop=%v",
			standard, desktop,
		)
	}
	standard, desktop = classifyAIKBTemplate(StandardTemplateName, DefaultDesktopDockerImage)
	if !standard || desktop {
		t.Fatalf(
			"named CLI template must stay CLI even if the image tag is desktop, got standard=%v desktop=%v",
			standard, desktop,
		)
	}
}

func TestClassifyAIKBTemplateNamelessImageUsesTag(t *testing.T) {
	standard, desktop := classifyAIKBTemplate("", DefaultDockerImage)
	if !standard || desktop {
		t.Fatalf("CLI image with no name must be standard, got standard=%v desktop=%v", standard, desktop)
	}
	standard, desktop = classifyAIKBTemplate("", DefaultDesktopDockerImage)
	if standard || !desktop {
		t.Fatalf("desktop image with no name must be desktop, got standard=%v desktop=%v", standard, desktop)
	}
	standard, desktop = classifyAIKBTemplate("", DefaultCubeDesktopTemplateImage)
	if standard || !desktop {
		t.Fatalf("Cube desktop image with no name must be desktop, got standard=%v desktop=%v", standard, desktop)
	}
}
