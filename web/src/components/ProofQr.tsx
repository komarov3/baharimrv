import { useEffect, useState } from 'react';
import QRCode from 'qrcode';

export function ProofQr({ proofId }: { proofId: string }) {
  const [dataUrl, setDataUrl] = useState('');
  const url = `${window.location.origin}/proof/${encodeURIComponent(proofId)}`;
  useEffect(() => {
    let active = true;
    QRCode.toDataURL(url, { width: 220, margin: 1, errorCorrectionLevel: 'M', color: { dark: '#12263b', light: '#ffffff' } })
      .then((value) => { if (active) setDataUrl(value); })
      .catch(() => { if (active) setDataUrl(''); });
    return () => { active = false; };
  }, [url]);
  return <div className="proof-qr">{dataUrl ? <img src={dataUrl} alt={`QR code for public proof ${proofId}`} /> : <span>QR unavailable</span>}<small>Scan to independently verify</small><code>{url}</code></div>;
}
