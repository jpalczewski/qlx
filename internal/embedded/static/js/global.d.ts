interface Window {
    qlx: {
        t(key: string): string;
        showToast(message: string, isError?: boolean): void;
        clearSelection(): void;
        initBulkSelect(): void;
        selectionEntries(): Array<{id: string, type: string}>;
        selectionSize(): number;
        selectionHas(id: string): boolean;
        openMovePicker(): void;
        openTagPicker(): void;
        openDeleteConfirm(): void;
        filterTemplates(printerSelectId: string, templateSelectId: string): void;
    };
}
