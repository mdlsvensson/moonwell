namespace Moonwell.Testing
{
    /// <summary>A burn that logs its unit's death.</summary>
    public class DyingBurn : Burn
    {
        /// <summary>Logs that the burn's unit died.</summary>
        public override void OnDeath(Unit? killer)
        {
            TestLog.Lines.Add("burn died");
        }
    }
}
