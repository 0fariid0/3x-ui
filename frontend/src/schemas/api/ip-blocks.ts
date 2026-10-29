import { z } from 'zod';
import { IPBlockChangeSchema, IPBlockStatusSchema } from '@/generated/zod';

export { IPBlockStatusSchema };
export type IPBlockStatus = z.infer<typeof IPBlockStatusSchema>;

export const IPBlockAddressSchema = z.union([z.ipv4(), z.ipv6()]).transform((value) => {
  if (!value.includes(':')) return value;
  const normalized = new URL(`http://[${value}]/`).hostname.slice(1, -1);
  const mapped = /^::ffff:([a-f\d]{1,4}):([a-f\d]{1,4})$/i.exec(normalized);
  if (!mapped) return normalized;
  const high = Number.parseInt(mapped[1], 16);
  const low = Number.parseInt(mapped[2], 16);
  return `${high >> 8}.${high & 255}.${low >> 8}.${low & 255}`;
});

export function canonicalBlockIP(value: string): string | null {
  const parsed = IPBlockAddressSchema.safeParse(value.trim());
  return parsed.success ? parsed.data : null;
}

export const IPBlockMutationSchema = IPBlockChangeSchema.extend({
  nodeId: z.number().int().min(0),
  ip: IPBlockAddressSchema,
  action: z.enum(['ban', 'unban']),
  email: z.string().max(255),
});

export type IPBlockMutation = z.infer<typeof IPBlockMutationSchema>;
