namespace Moonwell.Testing
{
    /// <summary>A unit that logs when it leaves, for the hook cases.</summary>
    public class LeaveLogger : Unit
    {
        /// <summary>Logs that the unit left.</summary>
        protected override void OnLeave()
        {
            TestLog.Lines.Add("leave");
        }
    }
}
