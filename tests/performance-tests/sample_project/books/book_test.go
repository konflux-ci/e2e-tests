package books_test

import (
	. "books"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Book", func() {
	var (
		longBook  Book
		shortBook Book
	)

	BeforeEach(func() {
		longBook = Book{
			Title:  "Les Miserables",
			Author: "Victor Hugo",
			Pages:  2783,
		}

		shortBook = Book{
			Title:  "Fox In Socks",
			Author: "Dr. Seuss",
			Pages:  24,
		}
	})

	// Add this code snippet inside the Describe block
	var currentTestDescription string

	BeforeEach(func() {
		currentTestDescription = CurrentGinkgoTestDescription().TestText
		fmt.Print(currentTestDescription + " - ")
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			fmt.Println("FAILED")
		} else {
			fmt.Println("PASSED")
		}
	})

	Describe("Categorizing book length", func() {
		Context("With more than 300 pages", func() {
			It("should be a novel", func() {
				category := longBook.CategoryByLength()
				fmt.Println("Testing book: " + longBook.Title + " - is a " + category)
				Expect(category).To(Equal("NOVEL"))
			})
		})

		Context("With fewer than 300 pages", func() {
			It("should be a short story", func() {
				category := shortBook.CategoryByLength()
				fmt.Println("Testing book: " + shortBook.Title + " - is a " + category)
				Expect(category).To(Equal("SHORT STORY"))
			})
		})
	})
})
