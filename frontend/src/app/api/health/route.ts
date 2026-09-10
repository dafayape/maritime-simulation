// Liveness probe internal frontend — dipakai HEALTHCHECK Dockerfile dan
// healthcheck docker-compose. Tidak menyentuh backend: hanya membuktikan
// proses Next.js standalone masih melayani request.
export const dynamic = 'force-dynamic';

export function GET(): Response {
  return Response.json({ status: 'ok', service: 'maritime-lora-mesh-dashboard' });
}
