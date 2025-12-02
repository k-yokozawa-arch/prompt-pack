import type { components } from './audit-zip.types'

export type AuditZipRequest = components['schemas']['AuditZipRequest']
export type AuditZipJob = components['schemas']['AuditZipJob']
export type AuditZipResult = components['schemas']['AuditZipResult']
export type InternalError = components['schemas']['InternalError']
export type AuditZipStatus = AuditZipJob['status']
export type ValidationError = components['schemas']['ValidationError']
export type ConflictError = components['schemas']['ConflictError']
export type RequestTooLargeError = components['schemas']['RequestTooLargeError']
export type RateLimitError = components['schemas']['RateLimitError']
export type SplitHint = components['schemas']['SplitHint']
