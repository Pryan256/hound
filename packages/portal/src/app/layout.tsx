import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Hound — Developer Dashboard",
  description: "Connect financial accounts to your app",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
