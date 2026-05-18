import { PageHeader, EmptyState } from "@/components/ui";

export function Placeholder({ section, title, body }: { section: string; title: string; body: string }) {
  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader section={section} title={title} />
      <EmptyState title="not yet wired" body={body} />
    </div>
  );
}
