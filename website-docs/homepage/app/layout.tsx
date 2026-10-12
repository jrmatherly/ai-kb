import type { Metadata } from "next";
import "./globals.css";
import { themeInitializationScript } from "./theme";

export const metadata: Metadata = {
  title: "AI KB - Find the answers, then put knowledge to work",
  description: "AI KB is an open source knowledge framework that combines RAG Q&A, agent reasoning and an automatic Wiki. v0.8.2 lets agents drive the local browser, publishes knowledge bases as an MCP server, and adds conversation forks and rewind. It runs on your own infrastructure.",
  // Reuse the documentation favicon so the homepage adds nothing at the site root.
  icons: { icon: "/docs/favicon.ico" },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="h-full antialiased" suppressHydrationWarning>
      <head><script dangerouslySetInnerHTML={{ __html: themeInitializationScript }} /></head>
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
