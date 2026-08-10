import type { ColumnDef } from '@tanstack/vue-table';
import { useTable } from '@tanstack/vue-table';
import { describe, expect, it } from 'vitest';
import { dataTableFeatures } from './features';

interface Row {
  name: string;
  owner: string;
}

const columns: ColumnDef<typeof dataTableFeatures, Row>[] = [
  { accessorKey: 'name' },
  {
    accessorKey: 'owner',
    // v9 only filters a column that declares a filterFn; an unset filterFn no
    // longer falls back to 'auto'. Every filterable column in the app declares
    // one explicitly, so this mirrors production.
    filterFn: (row, id, value: string) => row.getValue(id) === value,
  },
];

const data: Row[] = [
  { name: 'a', owner: 'alice' },
  { name: 'b', owner: 'bob' },
  { name: 'c', owner: 'alice' },
];

function createTable() {
  return useTable({ features: dataTableFeatures, data, columns });
}

describe('dataTableFeatures', () => {
  it('builds a core row model', () => {
    expect(createTable().getRowModel().rows).toHaveLength(3);
  });

  it('exposes faceted unique values, which drive the filter dropdowns', () => {
    const facets = createTable().getColumn('owner')!.getFacetedUniqueValues();

    expect([...facets.keys()].toSorted()).toEqual(['alice', 'bob']);
    expect(facets.get('alice')).toBe(2);
  });

  it('filters rows through the filtered row model', () => {
    const table = createTable();
    table.getColumn('owner')!.setFilterValue('alice');

    expect(table.getRowModel().rows.map((row) => row.original.name)).toEqual(['a', 'c']);
  });

  it('sorts rows through the sorted row model', () => {
    const table = createTable();
    table.getColumn('name')!.toggleSorting(true);

    expect(table.getRowModel().rows.map((row) => row.original.name)).toEqual(['c', 'b', 'a']);
  });
});
