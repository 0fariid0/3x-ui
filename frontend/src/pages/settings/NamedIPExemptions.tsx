import { Button, Input, InputNumber, Space, Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import { usePanelDateTime } from '@/hooks/usePanelDateTime';
type Rule = { name: string; address: string; expiresAt: number };
export default function NamedIPExemptions({value='',onChange,disabled}:{value?:string;onChange?:(v:string)=>void;disabled?:boolean}){
 const {t}=useTranslation();const text=(k:string)=>t(`pages.settings.ipBlocking.${k}`);const date=usePanelDateTime();
 let rules:Rule[]=[];try{const parsed=JSON.parse(value||'[]');if(Array.isArray(parsed))rules=parsed;}catch{ /* Backend validates stored input. */ }
 const change=(i:number,patch:Partial<Rule>)=>onChange?.(JSON.stringify(rules.map((r,k)=>k===i?{...r,...patch}:r)));
 return <Space orientation="vertical" style={{width:'100%'}}>{rules.map((r,i)=><Space key={i} wrap>
 <Input aria-label={text('exemptionName')} placeholder={text('exemptionName')} value={r.name} maxLength={128} disabled={disabled} onChange={e=>change(i,{name:e.target.value})}/>
 <Input aria-label={text('exemptionAddress')} placeholder={text('exemptionAddress')} value={r.address} maxLength={128} dir="ltr" disabled={disabled} onChange={e=>change(i,{address:e.target.value})}/>
 <InputNumber aria-label={text('exemptionMinutes')} placeholder={text('exemptionMinutes')} value={r.expiresAt ? Math.max(0,Math.ceil((r.expiresAt-Date.now())/60000)):0} min={0} max={525600} disabled={disabled} onChange={v=>change(i,{expiresAt:v&&v>0?Date.now()+v*60000:0})}/>
 <Tag color={r.expiresAt && r.expiresAt < Date.now() ? 'red' : undefined}>{r.expiresAt?date.formatDateTime(r.expiresAt):text('permanent')}</Tag>
 <Button danger disabled={disabled} onClick={()=>onChange?.(JSON.stringify(rules.filter((_,k)=>k!==i)))}>{text('remove')}</Button>
 </Space>)}<Button disabled={disabled||rules.length>=128} onClick={()=>onChange?.(JSON.stringify([...rules,{name:'',address:'',expiresAt:0}]))}>{text('addExemption')}</Button></Space>;
}
