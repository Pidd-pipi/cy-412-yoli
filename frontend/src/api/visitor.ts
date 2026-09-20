import {request} from '../utils/request';
import type {VisitorPass} from '../types';

export interface CreateVisitorPayload{plate:string;visitor_name?:string;building:string;room:string;start_at:string;end_at:string}

export const listVisitorPasses=(status?:string)=>request<VisitorPass[]>(`/visitor-passes${status?`?status=${encodeURIComponent(status)}`:''}`);
export const createVisitorPass=(data:CreateVisitorPayload)=>request<VisitorPass>('/visitor-passes',{method:'POST',body:JSON.stringify(data)});
export const approveVisitorPass=(id:number)=>request<VisitorPass>(`/visitor-passes/${id}/approve`,{method:'POST'});
export const rejectVisitorPass=(id:number,reason:string)=>request<VisitorPass>(`/visitor-passes/${id}/reject`,{method:'POST',body:JSON.stringify({reason})});
export const revokeVisitorPass=(id:number)=>request<VisitorPass>(`/visitor-passes/${id}/revoke`,{method:'POST'});
