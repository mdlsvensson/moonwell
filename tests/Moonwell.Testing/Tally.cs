namespace Moonwell.Testing
{
    /// <summary>A unit component that logs when it is added and when it is removed.</summary>
    public class Tally : UnitComponent
    {
        /// <summary>Logs that the tally was added.</summary>
        protected internal override void OnAdded()
        {
            TestLog.Lines.Add("tally added");
        }

        /// <summary>Logs that the tally was removed.</summary>
        protected internal override void OnRemoved()
        {
            TestLog.Lines.Add("tally removed");
        }
    }
}
