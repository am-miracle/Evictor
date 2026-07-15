import type { Metadata } from "next";
import "./globals.css";
import { IBM_Plex_Sans, Geist } from "next/font/google";
import { cn } from "@/lib/utils";
import { ThemeToggle } from "@/components/theme-toggle";

const geistHeading = Geist({ subsets: ["latin"], variable: "--font-heading" });

const ibmPlexSans = IBM_Plex_Sans({ subsets: ["latin"], variable: "--font-sans" });

const themeScript = `
(() => {
  try {
    const preference = localStorage.getItem("evictor-theme") || "system";
    const resolved = preference === "system"
      ? (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light")
      : preference;
    document.documentElement.classList.remove("light", "dark");
    document.documentElement.classList.add(resolved);
    document.documentElement.dataset.themePreference = preference;
  } catch (_) {}
})();`;

export const metadata: Metadata = {
  title: "Evictor",
  description: "Observability for serverless GPU inference",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={cn("font-sans", ibmPlexSans.variable, geistHeading.variable)}
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body>
        <ThemeToggle />
        {children}
      </body>
    </html>
  );
}
