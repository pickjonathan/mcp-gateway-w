// S3 operations against the shared ministack with explicit per-account credentials.
// Used both to seed buckets and to VALIDATE per-account state server-side (which
// account a given MCP actually wrote to).
import {
  S3Client,
  CreateBucketCommand,
  DeleteBucketCommand,
  HeadObjectCommand,
  ListBucketsCommand,
  ListObjectsV2Command,
  HeadBucketCommand,
} from "@aws-sdk/client-s3";
import { CONFIG, TenantCfg } from "./config.js";

export function s3For(accessKeyId: string, secretAccessKey: string): S3Client {
  return new S3Client({
    region: CONFIG.region,
    endpoint: CONFIG.awsEndpointHost,
    forcePathStyle: true,
    credentials: { accessKeyId, secretAccessKey },
  });
}

export async function createBucket(t: TenantCfg): Promise<void> {
  const s3 = s3For(t.accountId, t.secretAccessKey);
  try {
    await s3.send(new CreateBucketCommand({ Bucket: t.bucket }));
  } catch (e: any) {
    if (!/BucketAlreadyOwnedByYou|BucketAlreadyExists/.test(String(e?.name))) throw e;
  }
}

export async function deleteBucketBestEffort(t: TenantCfg): Promise<void> {
  try {
    await s3For(t.accountId, t.secretAccessKey).send(new DeleteBucketCommand({ Bucket: t.bucket }));
  } catch {
    /* ignore */
  }
}

/** Bucket names visible to an account (its namespace) — server-side attribution. */
export async function listBuckets(accessKeyId: string, secret: string): Promise<string[]> {
  const r = await s3For(accessKeyId, secret).send(new ListBucketsCommand({}));
  return (r.Buckets || []).map((b) => b.Name as string);
}

export async function listObjects(accessKeyId: string, secret: string, bucket: string): Promise<string[]> {
  const r = await s3For(accessKeyId, secret).send(new ListObjectsV2Command({ Bucket: bucket }));
  return (r.Contents || []).map((o) => o.Key as string);
}

/** True iff `key` exists in `bucket` for the given account (marker validation). */
export async function objectExists(accessKeyId: string, secret: string, bucket: string, key: string): Promise<boolean> {
  try {
    await s3For(accessKeyId, secret).send(new HeadObjectCommand({ Bucket: bucket, Key: key }));
    return true;
  } catch {
    return false;
  }
}

/** True iff `bucket` is reachable with the given credentials (preflight). */
export async function bucketAccessible(accessKeyId: string, secret: string, bucket: string): Promise<boolean> {
  try {
    await s3For(accessKeyId, secret).send(new HeadBucketCommand({ Bucket: bucket }));
    return true;
  } catch {
    return false;
  }
}
