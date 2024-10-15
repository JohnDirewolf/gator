package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/JohnDirewolf/gator/internal/config"
	"github.com/JohnDirewolf/gator/internal/database"

	_ "github.com/lib/pq"
)

type state struct {
	dbQueries *database.Queries
	configPtr *config.Config
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.dbQueries.GetUser(context.Background(), s.configPtr.CurrentUserName)
		if err != nil {
			return err
		}
		return handler(s, cmd, user)
	}
}

func convertToNullTime(dateString string) sql.NullTime {
	if dateString == "" {
		return sql.NullTime{Valid: false}
	}

	layout := "2006-01-02T15:04:05Z07:00" // Example layout, change as needed
	parsedTime, err := time.Parse(layout, dateString)
	if err != nil {
		//We don'tcare, then just send its Null
		return sql.NullTime{Valid: false}
	}

	return sql.NullTime{Time: parsedTime, Valid: true}
}

func scrapeFeeds(s *state) error {
	feedToFetch, err := s.dbQueries.GetNextFeedToFetch(context.Background())
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting next feed to fetch: %v", err))
	}

	err = s.dbQueries.MarkFeedFetched(context.Background(), feedToFetch.ID)
	if err != nil {
		return errors.New(fmt.Sprintf("Error marking feed as fetched: %v", err))
	}

	fmt.Println("Fetching Feeds")
	feed, err := fetchFeed(context.Background(), feedToFetch.Url)
	if err != nil {
		return err
	}
	//Process XML
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	//fmt.Println(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := 0; i < len(feed.Channel.Item); i++ {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		//fmt.Println("     " + feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)

		//Create the posts parameters
		createPostArgs := database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Title:       feed.Channel.Item[i].Title,
			Url:         feed.Channel.Link,
			Description: feed.Channel.Item[i].Description,
			PublishedAt: convertToNullTime(feed.Channel.Item[i].PubDate),
			FeedID:      feedToFetch.ID,
		}

		err = s.dbQueries.CreatePost(context.Background(), createPostArgs)
		if err != nil {
			if errors.Is(err, errors.New("unique constraint violation")) {
				//No problem.
				return nil
			}
			return fmt.Errorf("Failed to insert post: %w", err)
		}
	}
	return nil
}

// COMMANDS
type command struct {
	name string
	args []string
}

type commands struct {
	commandNameHandler map[string]func(*state, command) error
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.commandNameHandler[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	if handlerFunction, found := c.commandNameHandler[cmd.name]; found {
		err := handlerFunction(s, cmd)
		return err
	}
	return errors.New("Command not found. Have you RTFM?")
}

// COMMAND HANDLER FUNCTIONS
func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return errors.New("Missing user name, Proper usage: login <username>")
	}

	//Check if the user exists.
	// See if we have the user already.
	_, err := s.dbQueries.GetUser(context.Background(), cmd.args[0])
	if err != nil {
		//We successfully got the user.
		return errors.New("User does not exist. Please register username first!")
	}

	err = s.configPtr.SetUser(cmd.args[0])
	if err == nil {
		fmt.Printf("User has been set to: %s\n", cmd.args[0])
	}
	return err
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return errors.New("Missing user name, Proper usage: register <username>")
	}

	newUser := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.args[0],
	}

	// See if we have the user already.
	user, err := s.dbQueries.GetUser(context.Background(), newUser.Name)
	if err == nil {
		//We successfully got the user.
		return errors.New("User already exists")
	}

	user, err = s.dbQueries.CreateUser(context.Background(), newUser)
	if err == nil {
		fmt.Printf("User %s has been added,\n", user.Name)
		//fmt.Printf("User Data: %v, %v, %v, %v\n", user.ID, user.CreatedAt, user.UpdatedAt, user.Name)

		err := s.configPtr.SetUser(cmd.args[0])
		if err == nil {
			fmt.Printf("User has been set to: %s\n", cmd.args[0])
		}
		return err
	}

	return err
}

func handlerReset(s *state, cmd command) error {
	err := s.dbQueries.DeleteUsers(context.Background())
	if err == nil {
		//We successfully removed all users.
		fmt.Printf("All users purged! Hope you really meant to do that!\n")
	}
	return err
}

func handlerListUsers(s *state, cmd command) error {
	userList, err := s.dbQueries.ListUsers(context.Background())
	if err == nil {
		//We successfully got a list of names. Display
		for i := 0; i < len(userList); i++ {
			fmt.Printf(userList[i])
			if userList[i] == s.configPtr.CurrentUserName {
				fmt.Printf(" (current)")
			}
			fmt.Println("")
		}
	}
	return err
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return errors.New("Missing time span between fetches, Proper usage: agg <time span>")
	}

	timeBetweenRequests, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return fmt.Errorf("Unknown duration format: %v", err)
	}

	ticker := time.NewTicker(timeBetweenRequests)
	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}

	//This is unreachable as the course just creates an infinite loop, but its here as best practice.
	return nil
}

func handlerCreateFeeds(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return errors.New("Missing user name and/or feed URL. Proper usage: addfeed <feed_name> <URL>")
	}

	user, err := s.dbQueries.GetUser(context.Background(), s.configPtr.CurrentUserName)
	if err != nil {
		//We could not find the user.
		return errors.New("Unknown user. Please register user first!")
	}

	feedArgs := database.AddFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.args[0],
		Url:       cmd.args[1],
		UserID:    user.ID,
	}

	feed, err := s.dbQueries.AddFeed(context.Background(), feedArgs)
	if err != nil {
		//We could not access the feed.
		return errors.New(fmt.Sprintf("Error accessing feed: %v", err))
	}
	fmt.Printf("Created Feed:\n ID: %v\n, CreatedAt: %v\n, Updated At: %v\n, Name: %v\n, Url: %v\n, UserID: %v\n", feed.ID, feed.CreatedAt, feed.UpdatedAt, feed.Name, feed.Url, feed.UserID)

	//This just moves the URL to the first arg spot for the handlerFollow command being called as part of CreateFeeds
	cmd.args[0] = cmd.args[1]
	handlerFollow(s, cmd, user)

	return nil
}

func handlerFeeds(s *state, cmd command) error {
	feedList, err := s.dbQueries.ListFeeds(context.Background())
	if err != nil {
		return errors.New(fmt.Sprintf("Error accessing feeds: %v", err))
	}
	//We successfully got a list of feeds. Display
	for i := 0; i < len(feedList); i++ {
		fmt.Printf("Feed Name: %v\n   Feed URL: %v\n   User: %v\n", feedList[i].Name, feedList[i].Url, feedList[i].UserName)
	}
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) == 0 {
		return errors.New("Missing feed URL, Proper usage: follow <URL>")
	}

	currentUser, err := s.dbQueries.GetUser(context.Background(), s.configPtr.CurrentUserName)
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting current user details: %v", err))
	}

	feedData, err := s.dbQueries.GetFeed(context.Background(), cmd.args[0])
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting feed details: %v", err))
	}

	createFeedFollowArgs := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    currentUser.ID,
		FeedID:    feedData.ID,
	}

	follow, err := s.dbQueries.CreateFeedFollow(context.Background(), createFeedFollowArgs)
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting creating the feed follow: %v", err))
	}

	fmt.Printf("Created follow for feed: %v, for user %v\n", follow.FeedName, follow.UserName)

	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	user, err := s.dbQueries.GetUser(context.Background(), s.configPtr.CurrentUserName)
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting user details: %v", err))
	}

	feedList, err := s.dbQueries.ListFeedsForUser(context.Background(), user.ID)
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting list of feeds for user: %v", err))
	}

	fmt.Printf("Feeds for user %v:\n", s.configPtr.CurrentUserName)
	for i := 0; i < len(feedList); i++ {
		fmt.Printf("Feed Name: %v\n   Feed URL: %v\n", feedList[i].FeedName, feedList[i].FeedUrl)
	}
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) == 0 {
		return errors.New("Missing feed URL, Proper usage: unfollow <URL>")
	}

	feed, err := s.dbQueries.GetFeed(context.Background(), cmd.args[0])
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting feed details: %v", err))
	}

	unfollowArgs := database.UnfollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	}

	err = s.dbQueries.Unfollow(context.Background(), unfollowArgs)
	if err != nil {
		return errors.New(fmt.Sprintf("Error unfollowing feed: %v", err))
	}
	fmt.Printf("Unfollowed feed: %v\n", feed.Name)
	return nil
}

func handlerBrowse(s *state, cmd command, user database.User) error {

	//default limit to 2
	var limit int = 2
	if len(cmd.args) > 0 {
		num, err := strconv.Atoi(cmd.args[0])
		if err == nil {
			//successful conversion so apply it, if there is an error we ignore and stick to default
			limit = num
		}
	}

	getPostsForUserArgs := database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  int32(limit),
	}

	posts, err := s.dbQueries.GetPostsForUser(context.Background(), getPostsForUserArgs)
	if err != nil {
		return fmt.Errorf("Error retrieving posts for user: %v", err)
	}

	for i := 0; i < len(posts); i++ {
		fmt.Printf("Title: %v\n     Description: %v\n     Published Date: %v\n     URL: %v\n", posts[i].Title, posts[i].Description, posts[i].PublishedAt, posts[i].Url)
	}
	return nil
}

func main() {
	//fmt.Println("Hello Gator!")

	curState := state{}
	tmpConfig, err := config.Read()
	if err != nil {
		fmt.Printf("Error in initial Read(): %v", err)
		os.Exit(1)
	}
	curState.configPtr = &tmpConfig

	//Initialize commands
	var curCommands commands
	curCommands.commandNameHandler = make(map[string]func(*state, command) error)
	//Add commands
	curCommands.register("login", handlerLogin)
	curCommands.register("register", handlerRegister)
	curCommands.register("reset", handlerReset)
	curCommands.register("users", handlerListUsers)
	curCommands.register("agg", handlerAgg)
	curCommands.register("addfeed", middlewareLoggedIn(handlerCreateFeeds))
	curCommands.register("feeds", handlerFeeds)
	curCommands.register("follow", middlewareLoggedIn(handlerFollow))
	curCommands.register("following", middlewareLoggedIn(handlerFollowing))
	curCommands.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	curCommands.register("browse", middlewareLoggedIn(handlerBrowse))

	//open connection to database
	db, err := sql.Open("postgres", curState.configPtr.DbUrl)
	if err != nil {
		fmt.Printf("Error in connecting to Database: %v", err)
		os.Exit(1)
	}
	dbQueries := database.New(db)
	curState.dbQueries = dbQueries

	//Process User Commandline inputs.
	cmdLine := os.Args
	if len(cmdLine) < 2 {
		fmt.Println("Incorrect number of arguments.")
		os.Exit(1)
	}
	cmdToExecute := command{
		name: cmdLine[1],
		args: cmdLine[2:],
	}
	err = curCommands.run(&curState, cmdToExecute)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
