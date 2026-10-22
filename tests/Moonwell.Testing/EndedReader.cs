namespace Moonwell.Testing
{
    /// <summary>A unit component that logs what HasEnded reads inside its OnRemoved.</summary>
    public class EndedReader : UnitComponent
    {
        protected override void OnRemoved()
        {
            TestLog.Lines.Add("in OnRemoved, ended " + HasEnded);
        }
    }

    /// <summary>Cases that check when an owned thing counts as ended.</summary>
    public static class EndedCases
    {
        /// <summary>HasEnded is false inside OnRemoved and true once Remove returns.</summary>
        public static void ReadInOnRemoved(FakeWorld world)
        {
            var actor = new TestActor();
            var reader = actor.Add(new EndedReader());
            TestLog.Lines.Clear();
            actor.Remove(reader);
            TestLog.Lines.Add("after Remove, ended " + reader.HasEnded);
            Expect.Equal("in OnRemoved, ended False", TestLog.Lines[0], "inside");
            Expect.Equal("after Remove, ended True", TestLog.Lines[1], "after");
        }
    }
}
