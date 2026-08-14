import { type ReactElement, type ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";
import { render, type RenderOptions } from "@testing-library/react";

export function RouterWrap({
  children,
  initialEntries = ["/"],
}: {
  children: ReactNode;
  initialEntries?: string[];
}) {
  return <MemoryRouter initialEntries={initialEntries}>{children}</MemoryRouter>;
}

export function renderWithRouter(
  ui: ReactElement,
  options?: Omit<RenderOptions, "wrapper"> & { initialEntries?: string[] },
) {
  const { initialEntries = ["/"], ...rest } = options ?? {};
  return render(ui, {
    wrapper: ({ children }) => <RouterWrap initialEntries={initialEntries}>{children}</RouterWrap>,
    ...rest,
  });
}
