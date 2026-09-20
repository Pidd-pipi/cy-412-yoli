import type {VisitorPassStatus,VisitorEffectiveState} from '../types';

export const VISITOR_STATUS:Record<Uppercase<VisitorPassStatus>,VisitorPassStatus>={PENDING:'pending',APPROVED:'approved',REJECTED:'rejected',REVOKED:'revoked'};

export const visitorStatusText:Record<VisitorPassStatus,string>={pending:'待审',approved:'已批准',rejected:'已拒绝',revoked:'已撤销'};

export const visitorStateText:Record<VisitorEffectiveState,string>={upcoming:'未生效',active:'通行中',expired:'已过期'};

// 徽标颜色与后端 VisitorPassStatus / EffectiveState 枚举一一对应。
export const visitorStatusTagType:Record<VisitorPassStatus,'warning'|'success'|'danger'|'info'>={pending:'warning',approved:'success',rejected:'danger',revoked:'info'};

export const visitorStateTagType:Record<VisitorEffectiveState,'info'|'success'|'info'>={upcoming:'info',active:'success',expired:'info'};

export const VISITOR_ERROR_CODE_CONFLICT=40901;
