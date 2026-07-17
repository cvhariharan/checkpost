<script lang="ts">
    import { onMount, untrack } from "svelte";
    import { page } from "$app/state";
    import { goto, pushState } from "$app/navigation";
    import {
        deleteAlertRule,
        deleteAlertTarget,
        fetchAlertRule,
        fetchAlertRules,
        fetchAlertTargets,
        testAlertTarget,
        type AlertRule,
        type AlertTarget,
    } from "$lib/api";
    import { formatTimestamp } from "$lib/util";
    import { severityVariant } from "$lib/severity";
    import ErrorMessage from "$lib/components/ErrorMessage.svelte";
    import Pagination from "$lib/components/Pagination.svelte";
    import Spinner from "$lib/components/Spinner.svelte";
    import ConfirmDialog from "$lib/components/ConfirmDialog.svelte";
    import ActionsMenu from "$lib/components/ActionsMenu.svelte";
    import AlertRuleFormDialog from "$lib/components/AlertRuleFormDialog.svelte";
    import AlertTargetFormDialog from "$lib/components/AlertTargetFormDialog.svelte";
    import AlertInstancesDialog from "$lib/components/AlertInstancesDialog.svelte";
    import { canFrom, me } from "$lib/auth";
    import Plus from "@lucide/svelte/icons/plus";

    type OatTabsElement = HTMLElement & { activeIndex: number };

    const countPerPage = 10;

    const tabSlugs = ["rules", "targets"];
    const activeTabIndex = $derived.by(() => {
        const i = tabSlugs.indexOf(page.url.hash.replace(/^#/, ""));
        return i >= 0 ? i : 0;
    });
    let tabs = $state<OatTabsElement>();
    let error = $state("");
    let notice = $state("");

    // Rules
    let rules = $state<AlertRule[]>([]);
    const rulePage = $derived(
        Math.max(1, Number(page.url.searchParams.get("rules")) || 1),
    );
    let rulePageCount = $state(1);
    let ruleTotalCount = $state(0);
    let loadingRules = $state(true);
    let ruleFormOpen = $state(false);
    let editingRule = $state<AlertRule | null>(null);
    let ruleDeleteOpen = $state(false);
    let selectedRule = $state<AlertRule | null>(null);
    let deletingRule = $state(false);
    let instancesRule = $state<AlertRule | null>(null);
    let instancesOpen = $state(false);

    // Targets
    let targets = $state<AlertTarget[]>([]);
    const targetPage = $derived(
        Math.max(1, Number(page.url.searchParams.get("targets")) || 1),
    );
    let targetPageCount = $state(1);
    let targetTotalCount = $state(0);
    let loadingTargets = $state(true);
    let targetFormOpen = $state(false);
    let editingTarget = $state<AlertTarget | null>(null);
    let targetDeleteOpen = $state(false);
    let selectedTarget = $state<AlertTarget | null>(null);
    let deletingTarget = $state(false);
    let testingId = $state("");

    const canCreateRule = $derived(canFrom($me, "alert_rule", "create"));
    const canUpdateRule = $derived(canFrom($me, "alert_rule", "update"));
    const canDeleteRule = $derived(canFrom($me, "alert_rule", "delete"));
    const canViewInstances = $derived(canFrom($me, "alert_instance", "view"));

    const canViewTarget = $derived(canFrom($me, "alert_target", "view"));
    const canCreateTarget = $derived(canFrom($me, "alert_target", "create"));
    const canUpdateTarget = $derived(canFrom($me, "alert_target", "update"));
    const canDeleteTarget = $derived(canFrom($me, "alert_target", "delete"));
    const canTestTarget = $derived(canFrom($me, "alert_target", "execute"));

    onMount(() => {
        void loadRules();
        if (canViewTarget) void loadTargets();
    });

    $effect(() => {
        if (!tabs || tabs.activeIndex === activeTabIndex) return;
        tabs.activeIndex = activeTabIndex;
    });

    // Reload when a page param changes (pagination links). The first run is the
    // initial mount, which the onMount loads already cover, so skip it. `untrack`
    // keeps the effect from depending on state read inside the loaders.
    let ruleReloadReady = false;
    $effect(() => {
        rulePage;
        if (!ruleReloadReady) {
            ruleReloadReady = true;
            return;
        }
        untrack(() => void loadRules());
    });

    let targetReloadReady = false;
    $effect(() => {
        targetPage;
        if (!targetReloadReady) {
            targetReloadReady = true;
            return;
        }
        if (canViewTarget) untrack(() => void loadTargets());
    });

    function setTab(index: number) {
        const slug = tabSlugs[index];
        // Skip if already on this tab: ot-tabs echoes activations back through
        // ot-tab-change, so guarding here keeps each switch to one history entry.
        if (page.url.hash.replace(/^#/, "") === slug) return;
        const url = new URL(page.url);
        url.hash = slug;
        pushState(url, {});
    }

    function handleTabChange(event: CustomEvent<{ index: number }>) {
        setTab(event.detail.index);
    }

    async function loadRules() {
        loadingRules = true;
        error = "";
        try {
            const data = await fetchAlertRules({
                page: rulePage,
                countPerPage,
            });
            rules = data.rules || [];
            rulePageCount = data.page_count || 1;
            ruleTotalCount = data.total_count || rules.length;
        } catch (err) {
            error = (err as Error).message || "Failed to fetch alert rules";
        } finally {
            loadingRules = false;
        }
    }

    async function loadTargets() {
        loadingTargets = true;
        error = "";
        try {
            const data = await fetchAlertTargets({
                page: targetPage,
                countPerPage,
            });
            targets = data.targets || [];
            targetPageCount = data.page_count || 1;
            targetTotalCount = data.total_count || targets.length;
        } catch (err) {
            error = (err as Error).message || "Failed to fetch alert targets";
        } finally {
            loadingTargets = false;
        }
    }

    function openCreateRule() {
        if (!canCreateRule) return;
        editingRule = null;
        ruleFormOpen = true;
    }

    function openEditRule(rule: AlertRule) {
        if (!canUpdateRule) return;
        editingRule = rule;
        ruleFormOpen = true;
    }

    function openInstancesModal(rule: AlertRule) {
        if (!canViewInstances) return;
        instancesRule = rule;
        instancesOpen = true;
    }

    const deepLinkRuleUuid = $derived(page.url.searchParams.get("rule"));
    let handledDeepLink = "";

    $effect(() => {
        if (!deepLinkRuleUuid || deepLinkRuleUuid === handledDeepLink) return;
        handledDeepLink = deepLinkRuleUuid;
        untrack(() => void openRuleByUuid(deepLinkRuleUuid));
    });

    async function openRuleByUuid(uuid: string) {
        setTab(0);
        try {
            const rule = await fetchAlertRule(uuid);
            if (canViewInstances) {
                openInstancesModal(rule);
            } else {
                openEditRule(rule);
            }
        } catch (err) {
            error = (err as Error).message || "Failed to load alert rule";
            clearDeepLink();
        }
    }

    function clearDeepLink() {
        handledDeepLink = "";
        if (!page.url.searchParams.has("rule")) return;
        const url = new URL(page.url);
        url.searchParams.delete("rule");
        void goto(url, { replaceState: true, keepFocus: true, noScroll: true });
    }

    async function handleRuleSaved() {
        ruleFormOpen = false;
        await loadRules();
    }

    function confirmDeleteRule(rule: AlertRule) {
        if (!canDeleteRule) return;
        selectedRule = rule;
        ruleDeleteOpen = true;
    }

    async function removeRule() {
        if (!selectedRule) return;
        deletingRule = true;
        error = "";
        try {
            await deleteAlertRule(selectedRule.uuid);
            ruleDeleteOpen = false;
            selectedRule = null;
            await loadRules();
        } catch (err) {
            error = (err as Error).message || "Failed to delete alert rule";
        } finally {
            deletingRule = false;
        }
    }

    function openCreateTarget() {
        if (!canCreateTarget) return;
        editingTarget = null;
        targetFormOpen = true;
    }

    function openEditTarget(target: AlertTarget) {
        if (!canUpdateTarget) return;
        editingTarget = target;
        targetFormOpen = true;
    }

    async function handleTargetSaved() {
        targetFormOpen = false;
        await loadTargets();
    }

    function confirmDeleteTarget(target: AlertTarget) {
        if (!canDeleteTarget) return;
        selectedTarget = target;
        targetDeleteOpen = true;
    }

    async function removeTarget() {
        if (!selectedTarget) return;
        deletingTarget = true;
        error = "";
        try {
            await deleteAlertTarget(selectedTarget.uuid);
            targetDeleteOpen = false;
            selectedTarget = null;
            await loadTargets();
        } catch (err) {
            error = (err as Error).message || "Failed to delete alert target";
        } finally {
            deletingTarget = false;
        }
    }

    async function sendTest(target: AlertTarget) {
        testingId = target.uuid;
        error = "";
        notice = "";
        try {
            await testAlertTarget(target.uuid);
            notice = `Test alert sent to ${target.name}`;
        } catch (err) {
            error = (err as Error).message || "Test delivery failed";
        } finally {
            testingId = "";
        }
    }
</script>

<section class="vstack gap-4">
    <header class="mb-4">
        <h1 class="mb-2">Alerts</h1>
        <p class="text-light">
            Send alerts to targets based on rule evaluations
        </p>
    </header>

    <ErrorMessage message={error} onClose={() => (error = "")} />
    {#if notice}
        <p class="badge" data-variant="success">{notice}</p>
    {/if}

    {#if $me}
    <ot-tabs
        bind:this={tabs}
        class="alerts-tabs"
        onot-tab-change={handleTabChange}
    >
        <div role="tablist" aria-label="Alert sections">
            <button
                type="button"
                role="tab"
                aria-selected={activeTabIndex === 0}
                onclick={() => setTab(0)}
            >
                Rules
            </button>
            {#if canViewTarget}
                <button
                    type="button"
                    role="tab"
                    aria-selected={activeTabIndex === 1}
                    onclick={() => setTab(1)}
                >
                    Targets
                </button>
            {/if}
        </div>

        <div role="tabpanel">
            <div class="vstack gap-3">
                <div class="hstack justify-between">
                    <div>
                        <h4 class="mb-2">Rules</h4>
                    </div>
                    {#if canCreateRule}
                        <button
                            type="button"
                            class="gap-1"
                            onclick={openCreateRule}
                        >
                            <Plus size={16} aria-hidden="true" />
                            Create Rule
                        </button>
                    {/if}
                </div>

                {#if loadingRules}
                    <Spinner />
                {:else}
                    <div class="table">
                        <table>
                            <thead>
                                <tr>
                                    <th>Name</th>
                                    <th>Source</th>
                                    <th>Severity</th>
                                    <th>Status</th>
                                    <th>Last evaluated</th>
                                    <th class="align-right"
                                        ><span class="sr-only">Actions</span
                                        ></th
                                    >
                                </tr>
                            </thead>
                            <tbody>
                                {#each rules as rule}
                                    <tr>
                                        <td>
                                            {#if canViewInstances}
                                                <button
                                                    type="button"
                                                    class="cell-link"
                                                    onclick={() =>
                                                        openInstancesModal(
                                                            rule,
                                                        )}
                                                >
                                                    {rule.name || "Untitled"}
                                                </button>
                                            {:else}
                                                <span
                                                    >{rule.name ||
                                                        "Untitled"}</span
                                                >
                                            {/if}
                                            {#if rule.description}<p
                                                    class="text-light"
                                                >
                                                    {rule.description}
                                                </p>{/if}
                                        </td>
                                        <td
                                            ><span class="badge outline"
                                                >{rule.source}</span
                                            ></td
                                        >
                                        <td>
                                            <span
                                                class="badge"
                                                data-variant={severityVariant[
                                                    rule.severity || "medium"
                                                ]}
                                            >
                                                {rule.severity}
                                            </span>
                                        </td>
                                        <td>
                                            <span
                                                class="badge"
                                                data-variant={rule.enabled
                                                    ? "success"
                                                    : "warning"}
                                            >
                                                {rule.enabled
                                                    ? "Enabled"
                                                    : "Disabled"}
                                            </span>
                                        </td>
                                        <td
                                            >{formatTimestamp(
                                                rule.last_evaluated_at,
                                            )}</td
                                        >
                                        <td class="align-right">
                                            {#if canUpdateRule || canDeleteRule}
                                                <ActionsMenu
                                                    label={`Actions for ${rule.name || "rule"}`}
                                                >
                                                    {#if canUpdateRule}
                                                        <button
                                                            role="menuitem"
                                                            type="button"
                                                            onclick={() =>
                                                                openEditRule(
                                                                    rule,
                                                                )}>Edit</button
                                                        >
                                                    {/if}
                                                    {#if canUpdateRule && canDeleteRule}<hr
                                                        />{/if}
                                                    {#if canDeleteRule}
                                                        <button
                                                            role="menuitem"
                                                            type="button"
                                                            onclick={() =>
                                                                confirmDeleteRule(
                                                                    rule,
                                                                )}
                                                            >Delete</button
                                                        >
                                                    {/if}
                                                </ActionsMenu>
                                            {/if}
                                        </td>
                                    </tr>
                                {:else}
                                    <tr>
                                        <td
                                            colspan="6"
                                            class="align-center text-light"
                                            >No alert rules found</td
                                        >
                                    </tr>
                                {/each}
                            </tbody>
                        </table>
                    </div>

                    <footer class="hstack justify-between">
                        <p class="text-light">
                            <strong>{ruleTotalCount}</strong> rules
                        </p>
                        <Pagination
                            currentPage={rulePage}
                            pageCount={rulePageCount}
                            param="rules"
                        />
                    </footer>
                {/if}
            </div>
        </div>

        {#if canViewTarget}
            <div role="tabpanel">
                <div class="vstack gap-3">
                    <div class="hstack justify-between">
                        <div>
                            <h4 class="mb-2">Targets</h4>
                        </div>
                        {#if canCreateTarget}
                            <button
                                type="button"
                                class="gap-1"
                                onclick={openCreateTarget}
                            >
                                <Plus size={16} aria-hidden="true" />
                                Create Target
                            </button>
                        {/if}
                    </div>

                    {#if loadingTargets}
                        <Spinner />
                    {:else}
                        <div class="table">
                            <table>
                                <thead>
                                    <tr>
                                        <th>Name</th>
                                        <th>Type</th>
                                        <th>Status</th>
                                        <th>Updated</th>
                                        <th class="align-right"
                                            ><span class="sr-only">Actions</span
                                            ></th
                                        >
                                    </tr>
                                </thead>
                                <tbody>
                                    {#each targets as target}
                                        <tr>
                                            <td>
                                                <button
                                                    type="button"
                                                    class="cell-link"
                                                    onclick={() =>
                                                        openEditTarget(target)}
                                                >
                                                    {target.name || "Untitled"}
                                                </button>
                                            </td>
                                            <td
                                                ><span class="badge outline"
                                                    >{target.type}</span
                                                ></td
                                            >
                                            <td>
                                                <span
                                                    class="badge"
                                                    data-variant={target.enabled
                                                        ? "success"
                                                        : "warning"}
                                                >
                                                    {target.enabled
                                                        ? "Enabled"
                                                        : "Disabled"}
                                                </span>
                                            </td>
                                            <td
                                                >{formatTimestamp(
                                                    target.updated_at,
                                                )}</td
                                            >
                                            <td class="align-right">
                                                {#if canUpdateTarget || canDeleteTarget || canTestTarget}
                                                    <ActionsMenu
                                                        label={`Actions for ${target.name || "target"}`}
                                                    >
                                                        {#if canTestTarget}
                                                            <button
                                                                role="menuitem"
                                                                type="button"
                                                                disabled={testingId ===
                                                                    target.uuid}
                                                                onclick={() =>
                                                                    sendTest(
                                                                        target,
                                                                    )}
                                                            >
                                                                {testingId ===
                                                                target.uuid
                                                                    ? "Sending..."
                                                                    : "Send test"}
                                                            </button>
                                                        {/if}
                                                        {#if canUpdateTarget}
                                                            <button
                                                                role="menuitem"
                                                                type="button"
                                                                onclick={() =>
                                                                    openEditTarget(
                                                                        target,
                                                                    )}
                                                                >Edit</button
                                                            >
                                                        {/if}
                                                        {#if canDeleteTarget}<hr
                                                            />
                                                            <button
                                                                role="menuitem"
                                                                type="button"
                                                                onclick={() =>
                                                                    confirmDeleteTarget(
                                                                        target,
                                                                    )}
                                                                >Delete</button
                                                            >
                                                        {/if}
                                                    </ActionsMenu>
                                                {/if}
                                            </td>
                                        </tr>
                                    {:else}
                                        <tr>
                                            <td
                                                colspan="5"
                                                class="align-center text-light"
                                                >No alert targets found</td
                                            >
                                        </tr>
                                    {/each}
                                </tbody>
                            </table>
                        </div>

                        <footer class="hstack justify-between">
                            <p class="text-light">
                                <strong>{targetTotalCount}</strong> targets
                            </p>
                            <Pagination
                                currentPage={targetPage}
                                pageCount={targetPageCount}
                                param="targets"
                            />
                        </footer>
                    {/if}
                </div>
            </div>
        {/if}
    </ot-tabs>
    {:else}
        <Spinner fill />
    {/if}
</section>

<AlertRuleFormDialog
    open={ruleFormOpen}
    rule={editingRule}
    onClose={() => {
        ruleFormOpen = false;
        clearDeepLink();
    }}
    onSaved={handleRuleSaved}
/>

<AlertInstancesDialog
    bind:open={instancesOpen}
    rule={instancesRule}
    onClose={() => {
        instancesRule = null;
        clearDeepLink();
    }}
/>

<AlertTargetFormDialog
    open={targetFormOpen}
    target={editingTarget}
    onClose={() => (targetFormOpen = false)}
    onSaved={handleTargetSaved}
/>

<ConfirmDialog
    bind:open={ruleDeleteOpen}
    title="Delete Alert Rule"
    message="Are you sure you want to delete this alert rule? This action cannot be undone."
    confirming={deletingRule}
    confirmingLabel="Deleting..."
    onConfirm={removeRule}
    onCancel={() => (selectedRule = null)}
/>

<ConfirmDialog
    bind:open={targetDeleteOpen}
    title="Delete Alert Target"
    message="Are you sure you want to delete this alert target? This action cannot be undone."
    confirming={deletingTarget}
    confirmingLabel="Deleting..."
    onConfirm={removeTarget}
    onCancel={() => (selectedTarget = null)}
/>

<style>
    .alerts-tabs {
        display: block;
    }
</style>
