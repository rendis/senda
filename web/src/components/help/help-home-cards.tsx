import Link from "next/link";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { helpArticles } from "@/lib/help";

interface HelpHomeCardsProps {
  basePath: string;
}

export function HelpHomeCards({ basePath }: HelpHomeCardsProps) {
  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {helpArticles
        .filter((article) => article.key !== "index")
        .map((article) => (
          <Link key={article.key} href={`${basePath}/${article.key}`}>
            <Card className="h-full transition-colors hover:border-primary/40 hover:bg-accent/40">
              <CardHeader>
                <CardTitle>{article.title}</CardTitle>
                <CardDescription>{article.description}</CardDescription>
              </CardHeader>
              <CardContent className="pt-0 text-sm text-muted-foreground">
                Open guide
              </CardContent>
            </Card>
          </Link>
        ))}
    </div>
  );
}
