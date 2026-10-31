import { useState } from 'react';
import { Alert, Button, Descriptions, Input, Select, Space, Table, Tag, Typography } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';
import { HttpUtil } from '@/utils';
import { usePanelDateTime } from '@/hooks/usePanelDateTime';
const Schema=z.object({history:z.array(z.object({id:z.number(),email:z.string(),ip:z.string(),firstSeen:z.number(),lastSeen:z.number()})),checkedAt:z.number(),health:z.object({checkedAt:z.number(),lastSuccess:z.number(),available:z.boolean(),stale:z.boolean(),reason:z.string()}),checks:z.record(z.string(),z.string()),events:z.array(z.object({id:z.number(),email:z.string(),kind:z.string(),summary:z.string(),details:z.string(),createdAt:z.number()}))});
export default function IPLimitOverview({nodeId}:{nodeId:number}){
 const {t}=useTranslation();const text=(k:string)=>t(`pages.settings.ipBlocking.${k}`);const date=usePanelDateTime();const [search,setSearch]=useState('');const [hours,setHours]=useState(24);const [kind,setKind]=useState('all');
 const q=useQuery({queryKey:['ip-limit-overview',nodeId,search,hours],refetchInterval:15000,queryFn:async()=>{const r=await HttpUtil.get(`/panel/api/server/ipBlocks/overview?nodeId=${nodeId}&hours=${hours}&search=${encodeURIComponent(search)}`,undefined,{silent:true});if(!r?.success)throw new Error(r?.msg||text('failed'));return Schema.parse(r.obj);}});
 return <Space orientation="vertical" style={{width:'100%'}}><Typography.Title level={5}>{text('overview')}</Typography.Title>
 <Select value={hours} aria-label={text('timeRange')} onChange={setHours} options={[1,24,168,744].map(v=>({value:v,label:`${v} ${text('hours')}`}))}/><Select value={kind} aria-label={text('eventKind')} onChange={setKind} options={['all','decisions','ban','unban'].map(v=>({value:v,label:text(v)}))}/>
 <Input.Search allowClear placeholder={text('overviewSearch')} onSearch={setSearch}/><Button onClick={()=>void q.refetch()} loading={q.isFetching}>{text('refresh')}</Button>
 {q.isError?<Alert type="error" title={q.error.message}/>:q.data&&<>
 <Alert type={q.data.health.stale?'warning':'success'} title={text(q.data.health.stale?'staleHealth':'healthy')} description={`${text('lastSuccess')}: ${date.formatDateTime(q.data.health.lastSuccess)||'—'}`}/>
 <Descriptions size="small" bordered column={1} items={Object.entries(q.data.checks).map(([k,v])=>({key:k,label:text(k),children:v==='ready'?<Tag color="green">{text('ready')}</Tag>:<span dir="auto">{v}</span>}))}/>
 <Table size="small" rowKey="id" dataSource={q.data.events.filter(r=>kind==='all'||(kind==='decisions'?r.kind==='ip_limit_decision':kind==='unban'?r.kind.includes('unban'):r.kind.includes('ban')&&!r.kind.includes('unban')))} pagination={{pageSize:10}} columns={[
 {title:text('eventTime'),key:'time',render:(_v,record)=>date.formatDateTime(record.createdAt)},
 {title:text('account'),dataIndex:'email'},
 {title:text('decision'),key:'decision',render:(_v,record)=>text(record.summary)},
 ]} expandable={{expandedRowRender:record=>{try{const d=JSON.parse(record.details);return <Space orientation="vertical"><span>{text('limit')}: {d.limit??'—'}</span><span>{text('kept')}: {(d.kept||[]).map((v:{ip:string})=>v.ip).join(', ')||'—'}</span><span>{text('excess')}: {(d.excess||[]).map((v:{ip:string})=>v.ip).join(', ')||'—'}</span></Space>;}catch{return record.summary;}}}}/>
 <Typography.Title level={5}>{text('history')}</Typography.Title><Table size="small" rowKey="id" dataSource={q.data.history} pagination={{pageSize:10}} columns={[{title:text('account'),dataIndex:'email'},{title:text('address'),dataIndex:'ip',render:(ip:string)=><bdi dir="ltr">{ip}</bdi>},{title:text('firstSeen'),key:'first',render:(_v,r)=>date.formatDateTime(r.firstSeen)},{title:text('lastSeen'),key:'last',render:(_v,r)=>date.formatDateTime(r.lastSeen)}]}/>
 </>}
 </Space>;
}
