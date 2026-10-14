import type { Metadata } from "next";
import { Fraunces, Instrument_Sans, JetBrains_Mono } from "next/font/google";
import "./globals.css";

const display = Fraunces({ subsets: ["latin"], variable: "--font-display", display: "swap" });
const sans    = Instrument_Sans({ subsets: ["latin"], variable: "--font-sans",    display: "swap" });
const mono    = JetBrains_Mono({ subsets: ["latin"], variable: "--font-mono",     display: "swap" });

export const metadata: Metadata = {
  title: "Vaultside",
  description: "Multi-tenant LMS for financial services",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" data-theme="light" className={`${display.variable} ${sans.variable} ${mono.variable}`}>
      <body>{children}</body>
    </html>
  );
}