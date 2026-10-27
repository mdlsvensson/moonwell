namespace Moonwell.Testing
{
    /// <summary>A unit component with a helper of its own named like a reaction, with other parameters, and no reaction overridden.</summary>
    public sealed class Helpered : UnitComponent
    {
        /// <summary>What the helper was called with.</summary>
        public static string Log = "";

        /// <summary>A helper, not the OnDeath reaction.</summary>
        public void OnDeath(string reason)
        {
            Log += "helper(" + reason + ")";
        }
    }
}
