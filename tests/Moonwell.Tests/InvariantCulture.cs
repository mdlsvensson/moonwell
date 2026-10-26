using System.Globalization;
using System.Runtime.CompilerServices;

namespace Moonwell.Tests;

/// <summary>
/// The cases compare text with what the game's Lua writes: "12.5" and "-4". .NET writes numbers in the machine's format, "12,5"
/// and "−4" (a Unicode minus) on a Swedish one, so every test of this assembly runs under the invariant culture, set when the
/// assembly loads, before any test runs.
/// </summary>
internal static class InvariantCulture
{
    [ModuleInitializer]
    internal static void Set()
    {
        CultureInfo.DefaultThreadCurrentCulture = CultureInfo.InvariantCulture;
        CultureInfo.DefaultThreadCurrentUICulture = CultureInfo.InvariantCulture;
        CultureInfo.CurrentCulture = CultureInfo.InvariantCulture;
        CultureInfo.CurrentUICulture = CultureInfo.InvariantCulture;
    }
}
