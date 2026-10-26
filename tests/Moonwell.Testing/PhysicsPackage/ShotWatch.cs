namespace Moonwell.Testing
{
    /// <summary>A missile component that writes "watch impact" to the test log at each impact of its missile.</summary>
    public sealed class ShotWatch : MissileComponent
    {
        /// <summary>Logs the impact.</summary>
        public override void OnImpact(Impact impact)
        {
            TestLog.Lines.Add("watch impact");
        }
    }
}
