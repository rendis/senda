"use client";

import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { copyToClipboard } from "@/lib/utils";
import { toast } from "sonner";
import type { DnsRecord } from "@/types/domains";

interface DnsRecordTableProps {
  records: DnsRecord[];
}

export function DnsRecordTable({ records }: DnsRecordTableProps) {
  async function handleCopy(value: string) {
    const ok = await copyToClipboard(value);
    if (ok) {
      toast.success("Copied to clipboard");
    } else {
      toast.error("Failed to copy");
    }
  }

  if (records.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        No DNS records generated yet.
      </p>
    );
  }

  return (
    <div className="rounded-lg border bg-card">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="font-mono text-[11px] font-semibold tracking-wider text-muted-foreground">
              TYPE
            </TableHead>
            <TableHead className="font-mono text-[11px] font-semibold tracking-wider text-muted-foreground">
              NAME
            </TableHead>
            <TableHead className="font-mono text-[11px] font-semibold tracking-wider text-muted-foreground">
              VALUE
            </TableHead>
            <TableHead className="w-10" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {records.map((record, idx) => (
            <TableRow key={idx}>
              <TableCell className="font-mono text-xs font-medium">
                {record.type}
              </TableCell>
              <TableCell className="font-mono text-xs max-w-[200px] truncate">
                {record.name}
              </TableCell>
              <TableCell className="font-mono text-xs max-w-[300px] truncate">
                {record.value}
              </TableCell>
              <TableCell>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 w-7 p-0"
                  onClick={() => handleCopy(record.value)}
                >
                  <Copy className="h-3.5 w-3.5" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
