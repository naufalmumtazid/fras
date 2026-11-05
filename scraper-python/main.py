import asyncio
import io
import random
import zipfile
from datetime import datetime
from typing import List
from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel
import httpx
from bs4 import BeautifulSoup

app = FastAPI(title="API Otomatis Downloader & Analisis iXBRL IDX")


class EmitenRequest(BaseModel):
  kode_emiten: str  # Contoh: "AADI"


def generate_last_5_quarters_urls(kode_emiten: str) -> List[str]:
  """Menghasilkan tepat 5 URL laporan keuangan ke belakang:

  Q3 2026, Q2 2026, Q1 2026, Audit 2025, Q3 2025 (atau urutan yang sesuai).
  """
  # Berdasarkan waktu saat ini (November 2026), kita mulai dari Q3 2026 ke belakang
  # Urutan 5 periode mundur: [Q3 2026, Q2 2026, Q1 2026, Audit/Q4 2025, Q3 2025]
  target_periods = [
      ("TW3", 2026),
      ("TW2", 2026),
      ("TW1", 2026),
      ("Audit", 2025),  # Audit / Q4 menggunakan folder 'Audit'
      ("TW3", 2025),
  ]

  urls = []
  for q, y in target_periods:
    if q == "Audit":
      url = f"https://www.idx.co.id/Portals/0/StaticData/ListedCompanies/Corporate_Actions/New_Info_JSX/Jenis_Informasi/01_Laporan_Keuangan/02_Soft_Copy_Laporan_Keuangan//Laporan%20Keuangan%20Tahun%20{y}/Audit/{kode_emiten.upper()}/inlineXBRL.zip"
    else:
      url = f"https://www.idx.co.id/Portals/0/StaticData/ListedCompanies/Corporate_Actions/New_Info_JSX/Jenis_Informasi/01_Laporan_Keuangan/02_Soft_Copy_Laporan_Keuangan//Laporan%20Keuangan%20Tahun%20{y}/{q}/{kode_emiten.upper()}/inlineXBRL.zip"
    urls.append(url)

  return urls


@app.post("/analisis-emiten/")
async def analisis_emiten(req: EmitenRequest):
  """Endpoint utama untuk mendownload, mendeteksi, dan menganalisis 5 kuartal terakhir emiten."""
  urls = generate_last_5_quarters_urls(req.kode_emiten)

  headers = {
      "User-Agent": (
          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML,"
          " like Gecko) Chrome/120.0.0.0 Safari/537.36"
      ),
      "Accept": (
          "application/zip,application/octet-stream,text/html,application/xhtml+xml;q=0.9,*/*;q=0.8"
      ),
      "Referer": "https://www.idx.co.id/",
      "Accept-Language": "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7",
  }

  results = []

  async with httpx.AsyncClient(
      headers=headers, follow_redirects=True, timeout=60.0
  ) as client:
    for url in urls:
      try:
        # Jeda acak untuk mencegah 403 / rate limit
        await asyncio.sleep(random.uniform(1.5, 3.5))

        response = await client.get(url)

        if response.status_code != 200 or "<html" in response.text[:100].lower():
          results.append({
              "url": url,
              "status": (
                  f"Gagal/Tidak Tersedia (HTTP Status {response.status_code})"
              ),
          })
          continue

        zip_bytes = response.content
        if not zipfile.is_zipfile(io.BytesIO(zip_bytes)):
          results.append({
              "url": url,
              "status": "Gagal: File bukan arsip ZIP valid (Corrupted)",
          })
          continue

        extracted_files = []
        with zipfile.ZipFile(io.BytesIO(zip_bytes)) as z:
          for inner_filename in z.namelist():
            if inner_filename.lower().endswith(
                (".xml", ".xbrl", ".htm", ".html", ".xhtml")
            ):
              with z.open(inner_filename) as f:
                file_content = f.read()
                soup = BeautifulSoup(file_content, "html.parser")

                keuangan_summary = {}
                for tag in soup.find_all(
                    lambda t: t.name and "nonfraction" in t.name.lower()
                ):
                  tag_name = tag.get("name", "")
                  tag_value = tag.text.strip()
                  if tag_name:
                    short_name = tag_name.split(":")[-1].lower()
                    if any(
                        k in short_name
                        for k in [
                            "cash",
                            "revenue",
                            "profit",
                            "asset",
                            "liabilities",
                        ]
                    ):
                      keuangan_summary[short_name] = tag_value

                extracted_files.append({
                    "file_ixbrl": inner_filename,
                    "ringkasan_akun": keuangan_summary,
                })

        results.append({
            "url": url,
            "status": "Sukses",
            "data_analisis": extracted_files,
        })

      except Exception as e:
        results.append({"url": url, "status": f"Error: {str(e)}"})

  return JSONResponse(content={
      "emiten": req.kode_emiten.upper(),
      "total_url_diperiksa": len(urls),
      "hasil": results,
  })