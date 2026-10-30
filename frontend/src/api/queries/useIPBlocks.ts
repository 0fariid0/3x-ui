import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { HttpUtil } from '@/utils';
import { IPBlockMutationSchema, IPBlockStatusSchema, type IPBlockMutation } from '@/schemas/api/ip-blocks';

const blockKey = (nodeId: number) => ['server', 'ip-blocks', nodeId] as const;

export function useIPBlocks(open: boolean, nodeId: number) {
  const client = useQueryClient();
  const query = useQuery({
    queryKey: blockKey(nodeId),
    enabled: open,
    refetchInterval: open ? 15_000 : false,
    retry: false,
    queryFn: async () => {
      const response = await HttpUtil.get(`/panel/api/server/ipBlocks?nodeId=${nodeId}`, undefined, { silent: true });
      if (!response?.success) throw new Error(response?.msg || 'IP block status unavailable');
      return IPBlockStatusSchema.parse(response.obj);
    },
  });
  const mutation = useMutation({
    mutationFn: changeIPBlock,
    onSuccess: (status, request) => client.setQueryData(blockKey(request.nodeId), status),
    onError: (_error, request) => { void client.invalidateQueries({ queryKey: blockKey(request.nodeId) }); },
  });
  return { query, mutation };
}

export async function changeIPBlock(request: IPBlockMutation) {
  const payload = IPBlockMutationSchema.parse(request);
  const response = await HttpUtil.post('/panel/api/server/ipBlocks/change', payload, { silent: true, headers: { 'Content-Type': 'application/json' } });
  if (!response?.success) throw new Error(response?.msg || 'IP block operation failed');
  return IPBlockStatusSchema.parse(response.obj);
}
