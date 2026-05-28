<script lang="ts">
    import {
        fetchOsqueryBootstrapProfile,
        type OsqueryBootstrapPackage,
        type OsqueryBootstrapPlatform,
        type OsqueryBootstrapProfile,
    } from "$lib/api";
    import { toast } from "$lib/util";
    import ErrorMessage from "./ErrorMessage.svelte";
    import Spinner from "./Spinner.svelte";

    export let open = false;
    export let onClose: () => void = () => {};

    type OatTabsElement = HTMLElement & { activeIndex: number };

    let dialog: HTMLDialogElement;
    let tabs: OatTabsElement;
    let profile: OsqueryBootstrapProfile | null = null;
    let loading = false;
    let error = "";
    let loaded = false;
    let activeTabIndex = 0;

    $: platforms = profile?.platforms || [];

    $: if (tabs && tabs.activeIndex !== activeTabIndex) {
        tabs.activeIndex = activeTabIndex;
    }

    $: if (open && dialog) {
        if (!dialog.open) dialog.showModal();
        if (!loaded && !loading) void loadProfile();
    }

    $: if (!open && dialog) {
        if (dialog.open) dialog.close();
    }

    async function loadProfile() {
        loading = true;
        error = "";
        try {
            profile = await fetchOsqueryBootstrapProfile();
            loaded = true;
            activeTabIndex = 0;
        } catch (err) {
            error =
                (err as Error).message || "Failed to load osquery bootstrap";
        } finally {
            loading = false;
        }
    }

    function handleClose() {
        if (open) {
            open = false;
            onClose();
        }
    }

    function handleTabChange(event: CustomEvent<{ index: number }>) {
        activeTabIndex = event.detail.index;
    }

    async function copyText(value: string, label: string) {
        try {
            await navigator.clipboard.writeText(value);
            toast(`${label} copied`, "Clipboard", { variant: "success" });
        } catch {
            toast(`Could not copy ${label.toLowerCase()}`, "Clipboard", {
                variant: "danger",
            });
        }
    }

    function downloadScript(platform: OsqueryBootstrapPlatform) {
        const extension = platform.key === "windows" ? "ps1" : "sh";
        const blob = new Blob([platform.script], {
            type: "text/plain;charset=utf-8",
        });
        const url = URL.createObjectURL(blob);
        const link = document.createElement("a");
        link.href = url;
        link.download = `watcher-osquery-${platform.key}.${extension}`;
        document.body.appendChild(link);
        link.click();
        link.remove();
        URL.revokeObjectURL(url);
    }

    function commandLanguage(platform: OsqueryBootstrapPlatform): string {
        return platform.key === "windows" ? "powershell" : "bash";
    }

    function primaryPackage(
        platform: OsqueryBootstrapPlatform,
    ): OsqueryBootstrapPackage | undefined {
        return platform.package || platform.packages?.[0];
    }

    function packageSummary(pkg: OsqueryBootstrapPackage): string {
        return [pkg.format.toUpperCase(), pkg.architecture]
            .filter(Boolean)
            .join(" ");
    }
</script>

<dialog
    bind:this={dialog}
    class="bootstrap-dialog"
    closedby="any"
    onclose={handleClose}
>
    <header>
        <div>
            <h2>Install osquery</h2>
            <p>
                Run this command to install and enroll osquery into this Watcher
                instance.
            </p>
        </div>
    </header>

    <section class="vstack gap-4">
        <ErrorMessage message={error} onClose={() => (error = "")} />

        {#if loading && !profile}
            <Spinner />
        {:else if profile}
            <ot-tabs
                bind:this={tabs}
                class="bootstrap-tabs"
                onot-tab-change={handleTabChange}
            >
                <div role="tablist" aria-label="osquery bootstrap platforms">
                    {#each platforms as platform, index}
                        <button
                            type="button"
                            role="tab"
                            aria-selected={activeTabIndex === index}
                            onclick={() => (activeTabIndex = index)}
                        >
                            {platform.label}
                        </button>
                    {/each}
                </div>

                {#each platforms as platform}
                    <div role="tabpanel">
                        <div class="vstack gap-4">
                            <section class="vstack gap-2">
                                <div class="hstack justify-between">
                                    <div>
                                        <h3>Automated Install</h3>
                                        <p class="text-light">
                                            Installs osquery when missing, then
                                            writes Watcher enrollment config.
                                        </p>
                                    </div>
                                    <button
                                        type="button"
                                        onclick={() =>
                                            copyText(
                                                platform.command,
                                                "Command",
                                            )}
                                    >
                                        Copy command
                                    </button>
                                </div>
                                <pre data-code={commandLanguage(platform)}><code
                                        >{platform.command}</code
                                    ></pre>
                            </section>
                            <section>
                                <h4>What it does</h4>
                                <ul>
                                    <li>
                                        Installs osquery only if it is not
                                        already installed.
                                    </li>
                                    <li>
                                        Verifies package checksums before
                                        install.
                                    </li>
                                    <li>Writes Watcher enrollment config.</li>
                                    <li>
                                        Starts or restarts the osquery service.
                                    </li>
                                </ul>
                            </section>

                            <section class="vstack gap-4">
                                <div class="vstack gap-3">
                                    <h3>Manual install</h3>
                                    {#if platform.packages?.length}
                                        <div class="table">
                                            <table>
                                                <thead>
                                                    <tr>
                                                        <th>Package</th>
                                                        <th>Format</th>
                                                        <th>URL</th>
                                                        <th>SHA256</th>
                                                        <th class="align-right"
                                                            >Actions</th
                                                        >
                                                    </tr>
                                                </thead>
                                                <tbody>
                                                    {#each platform.packages as pkg}
                                                        <tr>
                                                            <td
                                                                ><strong
                                                                    >{pkg.label}</strong
                                                                ></td
                                                            >
                                                            <td
                                                                >{packageSummary(
                                                                    pkg,
                                                                )}</td
                                                            >
                                                            <td
                                                                ><code
                                                                    >{pkg.url ||
                                                                        "Not configured"}</code
                                                                ></td
                                                            >
                                                            <td
                                                                ><code
                                                                    >{pkg.sha256 ||
                                                                        "Not configured"}</code
                                                                ></td
                                                            >
                                                            <td
                                                                class="align-right"
                                                            >
                                                                <div
                                                                    class="hstack justify-end"
                                                                >
                                                                    <button
                                                                        type="button"
                                                                        class="small outline"
                                                                        onclick={() =>
                                                                            copyText(
                                                                                pkg.url,
                                                                                `${pkg.label} URL`,
                                                                            )}
                                                                        disabled={!profile.ready ||
                                                                            !pkg.url}
                                                                    >
                                                                        Copy URL
                                                                    </button>
                                                                    <button
                                                                        type="button"
                                                                        class="small outline"
                                                                        onclick={() =>
                                                                            copyText(
                                                                                pkg.sha256,
                                                                                `${pkg.label} checksum`,
                                                                            )}
                                                                        disabled={!profile.ready ||
                                                                            !pkg.sha256}
                                                                    >
                                                                        Copy
                                                                        checksum
                                                                    </button>
                                                                </div>
                                                            </td>
                                                        </tr>
                                                    {/each}
                                                </tbody>
                                            </table>
                                        </div>
                                    {:else if primaryPackage(platform)}
                                        {@const pkg = primaryPackage(platform)!}
                                        <p><strong>{pkg.label}</strong></p>
                                        <pre><code>{pkg.url}</code></pre>
                                        <pre><code>{pkg.sha256}</code></pre>
                                    {/if}

                                    <h4>Install steps</h4>
                                    <ol>
                                        {#each platform.install_steps || [] as step}
                                            <li>{step}</li>
                                        {/each}
                                    </ol>
                                </div>

                                <div class="vstack gap-3">
                                    <div class="hstack justify-between">
                                        <h4>Manual configuration</h4>
                                        <button
                                            type="button"
                                            class="small outline"
                                            onclick={() =>
                                                copyText(
                                                    platform.secret,
                                                    "Secret",
                                                )}
                                            disabled={!profile.ready}
                                        >
                                            Copy secret
                                        </button>
                                    </div>
                                    <p class="text-light">
                                        Secret file: <code
                                            >{platform.secret_path}</code
                                        >
                                    </p>
                                    <pre><code>{platform.secret}</code></pre>

                                    <div class="hstack justify-between">
                                        <p>
                                            <strong
                                                >{platform.flagfile_path}</strong
                                            >
                                        </p>
                                        <button
                                            type="button"
                                            class="small outline"
                                            onclick={() =>
                                                copyText(
                                                    platform.flagfile,
                                                    "Flags",
                                                )}
                                            disabled={!profile.ready}
                                        >
                                            Copy flags
                                        </button>
                                    </div>
                                    <pre><code>{platform.flagfile}</code></pre>

                                    <div class="hstack justify-between">
                                        <p><strong>Restart</strong></p>
                                        <button
                                            type="button"
                                            class="small outline"
                                            onclick={() =>
                                                copyText(
                                                    platform.restart_command,
                                                    "Restart command",
                                                )}
                                            disabled={!profile.ready}
                                        >
                                            Copy restart
                                        </button>
                                    </div>
                                    <pre><code>{platform.restart_command}</code
                                        ></pre>
                                </div>
                            </section>

                            <section class="vstack gap-2">
                                <h3>Verify</h3>
                                <pre><code>{platform.verify_command}</code
                                    ></pre>
                                <p class="text-light">
                                    Wait one check-in interval, then confirm the
                                    machine appears in Inventory.
                                </p>
                            </section>

                            <details>
                                <summary>Advanced details</summary>
                                <div class="vstack gap-2 mt-3">
                                    <p class="text-light">
                                        {platform.architecture_notes}
                                    </p>
                                    {#if platform.caveats?.length}
                                        <ul>
                                            {#each platform.caveats as caveat}
                                                <li>{caveat}</li>
                                            {/each}
                                        </ul>
                                    {/if}
                                    <div class="hstack justify-between">
                                        {#if profile.ready}
                                            <a
                                                href={platform.script_url}
                                                target="_blank"
                                                rel="noreferrer">Open script</a
                                            >
                                        {:else}
                                            <span class="text-light"
                                                >Script unavailable until
                                                configuration is ready</span
                                            >
                                        {/if}
                                        <button
                                            type="button"
                                            class="small outline"
                                            onclick={() =>
                                                downloadScript(platform)}
                                            disabled={!profile.ready}
                                        >
                                            Download script
                                        </button>
                                    </div>
                                    <pre><code>{platform.script}</code></pre>
                                </div>
                            </details>
                        </div>
                    </div>
                {/each}
            </ot-tabs>
        {/if}
    </section>

    <footer class="hstack justify-end">
        <button type="button" class="outline" onclick={() => dialog.close()}
            >Close</button
        >
    </footer>
</dialog>

<style>
    .bootstrap-dialog {
        width: min(78rem, 94vw);
    }

    .bootstrap-dialog > section {
        max-height: 72vh;
    }

    .bootstrap-tabs {
        display: block;
    }

    pre {
        overflow: auto;
        max-width: 100%;
        padding: var(--space-4);
        border: 1px solid var(--border);
        border-radius: var(--radius);
        background: var(--faint);
        white-space: pre-wrap;
        overflow-wrap: anywhere;
    }

    code {
        overflow-wrap: anywhere;
    }
</style>
