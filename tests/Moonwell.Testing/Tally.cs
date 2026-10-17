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

        /// <summary>Logs that the tally saw its unit pick up an item.</summary>
        public override void OnItemPickedUp(Item item)
        {
            TestLog.Lines.Add("tally got orb");
        }

        /// <summary>Logs that the tally saw its unit drop an item.</summary>
        public override void OnItemDropped(Item item)
        {
            TestLog.Lines.Add("tally dropped orb");
        }
    }
}
