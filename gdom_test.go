package gdom

import (
	"bytes"
	"testing"
)

func TestParse(t *testing.T) {
	var xmlstr = `<?xml version="1.0" encoding="UTF-8"?>
<beans xmlns="http://www.springframework.org/schema/beans" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:context="http://www.springframework.org/schema/context" xmlns:tx="http://www.springframework.org/schema/tx" xmlns:aop="http://www.springframework.org/schema/aop" xmlns:mybatis="http://mybatis.org/schema/mybatis-spring" xsi:schemaLocation="http://www.springframework.org/schema/beans http://www.springframework.org/schema/beans/spring-beans.xsd http://www.springframework.org/schema/context http://www.springframework.org/schema/context/spring-context.xsd http://www.springframework.org/schema/tx http://www.springframework.org/schema/tx/spring-tx-4.1.xsd http://www.springframework.org/schema/aop http://www.springframework.org/schema/aop/spring-aop-4.1.xsd
       http://mybatis.org/schema/mybatis-spring
       http://mybatis.org/schema/mybatis-spring.xsd">
    <tx:annotation-driven transaction-manager="transactionManager"/>

    <bean id="transactionManager" class="org.springframework.jdbc.datasource.DataSourceTransactionManager">
        <property name="dataSource" ref="orclDataSource"/>
    </bean>

          <!--
    <bean id="orclDataSource" class="org.apache.commons.dbcp.BasicDataSource"
          destroy-method="close">
        <property name="driverClassName" value="oracle.jdbc.driver.OracleDriver"/>
        <property name="url" value="xxxxxx"/>
        <property name="username" value="xxxxx"/>
        <property name="password" value="xxxxx"/>
        <property name="initialSize" value="111"/>
        <property name="maxActive" value="1111"/>
    </bean>
    -->

   	 <bean id="orclDataSource" class="org.apache.commons.dbcp.BasicDataSource"
          destroy-method="close">
        <property name="driverClassName" value="oracle.jdbc.driver.OracleDriver"/>
        <property name="url" value="xxxxxx"/>
        <property name="username" value="xxxxx"/>
        <property name="password" value="xxxxx"/>
        <property name="initialSize" value="111"/>
        <property name="maxActive" value="1111"/>
    </bean>
</beans>`
	xmldoc, err := ParseString(xmlstr)
	if err != nil {
		t.Error(err)
		return
	}
	bts := xmldoc.ToBytes()
	ss := string(bts)
	xmldoc, err = ParseString(ss)
	if err != nil {
		t.Error(err)
		return
	}
	sss := string(xmldoc.ToBytes())
	if ss != sss {
		t.Errorf("first->\n%s\nsecond->\n%s\n", ss, sss)
	}

}

func TestAdd(t *testing.T) {
	xmlstr := `<beans>
	<bean></bean>
</beans>`
	xmldoc, err := ParseString(xmlstr)
	if err != nil {
		t.Error(err)
		return
	}
	xmldoc.Root().AddNotParsedString("<xbean a='xxx'></xbean>")
	xbs := xmldoc.Root().Eles(NewName("", "xbean"))
	if len(xbs) != 1 {
		t.Error("wrong length")
		t.Error(xmldoc.ToString())
		return
	}
	xb := xbs[0]
	v, ok := xb.Value.(*Ele).GetAttr(NewName("", "a"))
	if !ok || v != "xxx" {
		t.Error("wrong attr value")
	}
}

func TestDel(t *testing.T) {
	xmlstr := `<beans id="super">
    <bean></bean>
    <xbean></xbean>
</beans>`
	xmldoc, err := ParseString(xmlstr)
	if err != nil {
		t.Error(err)
		return
	}
	xmldoc.Root().RemoveAttrByStrName("", "id")
	_, ok := xmldoc.Root().GetAttrByStrName("", "id")
	if ok {
		t.Error("delete attr failed")
		return
	}

	xmldoc.Root().RemoveEleByStrName("", "xbean")
	xbs := xmldoc.Root().ElesByStrName("", "xbean")
	if len(xbs) > 0 {
		t.Error("delete element failed")
		return
	}
}

func TestInsert(t *testing.T) {
	xmlstr := `<p><a/><b/><d/></p>`
	xmldoc, err := ParseString(xmlstr)
	if err != nil {
		t.Error(err)
		return
	}
	d := xmldoc.Root().ElesByStrName("", "d")[0]
	c := NewEle(NewName("", "c"), nil)
	err = xmldoc.Root().InsertBefore(c, d)
	if err != nil {
		t.Error(err)
		return
	}
	buf := bytes.NewBuffer(make([]byte, 0, 4))
	xmldoc.Root().IterNode(func(n Node) {
		e, ok := n.(*Ele)
		if ok {
			buf.Write([]byte(e.Name.Local))
		}
		return true
	})
	s := string(buf.Bytes())
	if s != "abcd" {
		t.Errorf("insert failed\n%s\n", s)
		t.Error(xmldoc.ToString())
		return
	}
}

func TestIterAttr(t *testing.T) {
	xmlstr := `<x a="1" b="2" c="3"></a>`
	d, _ := ParseString(xmlstr)
	buf := bytes.NewBuffer(make([]byte, 0, 8))
	d.Root().IterAttr(func(attr *Attr) {
		buf.Write([]byte(attr.Name.Local))
		buf.Write([]byte(attr.Value))
		return true
	})
	s := string(buf.Bytes())
	if s != "a1b2c3" {
		t.Error("iter failed")
	}
}

func TestIterDelEle(t *testing.T) {
	xmlstr := `<p a="1" b="2" c="3"><a/><b/><c/></p>`
	d, _ := ParseString(xmlstr)
	d.Root().IterNode(func(e Node) {
		ee, ok := e.(*Ele)
		if ok && "b" == ee.Name.Local {
			RemoveSelf(e)
		}
	})
	buf := bytes.NewBuffer(make([]byte, 0, 4))
	d.Root().IterNode(func(n Node) {
		e, ok := n.(*Ele)
		if ok {
			buf.Write([]byte(e.Name.Local))
		}
		return true
	})
	s := string(buf.Bytes())
	if s != "ac" {
		t.Error("iterdelele failed")
	}
}

func TestIterAddEle(t *testing.T) {
	xs := `<p><a/></p>`
	d, _ := ParseString(xs)
	r := d.Root()
	ele := NewEle(NewName("", "b"), nil)
	r.IterNode(func(n Node) {
		ne, ok := n.(*Ele)
		if ok && ne.Name.Local == "a" {
			r.InsertAfter(ele, n)
		}
		return true
	})
	xss := d.ToString()
	if xss != "<p><a/><b/></p>" {
		t.Error("iter insert after failed")
	}

	xs = `<p><a/></p>`
	d, _ = ParseString(xs)
	r = d.Root()
	ele = NewEle(NewName("", "b"), nil)
	r.IterNode(func(n Node) {
		ne, ok := n.(*Ele)
		if ok && ne.Name.Local == "a" {
			r.InsertBefore(ele, n)
		}
		return true
	})
	xss = d.ToString()
	if xss != "<p><b/><a/></p>" {
		t.Error("iter insert before failed")
	}
}

func TestAddCharData(t *testing.T) {
	xmlstr := `<p></p>`
	d, _ := ParseString(xmlstr)
	d.Root().AddCharDataStr("test")
	ss := d.ToString()
	if ss != "<p>test</p>" {
		t.Error("add chardata failed")
	}
}

func TestBeautiful(t *testing.T) {
	xs := `<p><a/><b><c></c><d/></b></p>`
	d, _ := ParseString(xs)
	rs := `<p>
    <a/>
    <b>
        <c/>
        <d/>
    </b>
</p>`
	d.Beautiful()
	rss := d.ToString()
	if rs != rss {
		t.Error("beautiful failed")
		t.Error(rss)
		t.Error(rs)
	}
}

func TestInsertString(t *testing.T) {
	xs := `<p>
	<a>
	</a>
	</p>`
	d, _ := ParseString(xs)
	d.Root().AddNotParsedString(`
		<b>
		</b>
		`)
	buf := bytes.NewBuffer([]byte(""))
	d.Root().IterNode(func(n Node) {
		e, ok := n.(*Ele)
		if ok {
			buf.Write([]byte(e.Name.Local))
		}
		return true
	})
	if buf.String() != "ab" {
		t.Error("add not parsed string failed")
	}

}
