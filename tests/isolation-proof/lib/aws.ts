// S3 operations against the emulator with explicit per-account credentials.
import {
  S3Client,
  CreateBucketCommand,
  DeleteBucketCommand,
  PutObjectCommand,
  GetObjectCommand,
  ListObjectsV2Command,
  HeadBucketCommand,
} from "@aws-sdk/client-s3";
import { CONFIG, TenantCfg } from "./config.js";

export function s3For(accessKeyId: string, secretAccessKey: string): S3Client {
  return new S3Client({
    region: CONFIG.region,
    endpoint: CONFIG.awsEndpoint,
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

export async function putObject(t: TenantCfg, key: string, body: string): Promise<void> {
  await s3For(t.accountId, t.secretAccessKey).send(
    new PutObjectCommand({ Bucket: t.bucket, Key: key, Body: body }),
  );
}

export async function getObject(t: TenantCfg, key: string): Promise<string> {
  const r = await s3For(t.accountId, t.secretAccessKey).send(
    new GetObjectCommand({ Bucket: t.bucket, Key: key }),
  );
  return await (r.Body as any).transformToString();
}

export async function listObjects(accessKeyId: string, secret: string, bucket: string): Promise<string[]> {
  const r = await s3For(accessKeyId, secret).send(new ListObjectsV2Command({ Bucket: bucket }));
  return (r.Contents || []).map((o) => o.Key as string);
}

/** True iff `bucket` is reachable with the given credentials (preflight + V4). */
export async function bucketAccessible(accessKeyId: string, secret: string, bucket: string): Promise<boolean> {
  try {
    await s3For(accessKeyId, secret).send(new HeadBucketCommand({ Bucket: bucket }));
    return true;
  } catch {
    return false;
  }
}
