namespace Moonwell.Testing
{
    /// <summary>An item component that logs when its item is picked up and dropped.</summary>
    public class OrbGlow : ItemComponent
    {
        /// <summary>Logs that the glow saw the pick-up.</summary>
        public override void OnPickedUp(Unit by)
        {
            TestLog.Lines.Add("glow picked");
        }

        /// <summary>Logs that the glow saw the drop.</summary>
        public override void OnDropped(Unit by)
        {
            TestLog.Lines.Add("glow dropped");
        }
    }
}
